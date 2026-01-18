package main

import (
	"bufio"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Stats struct {
	// Request metrics
	Requests          int64   `json:"requests"`
	Errors            int64   `json:"errors"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	SuccessRate       float64 `json:"success_rate_percent"`

	// Latency metrics
	AvgLatency  float64 `json:"avg_latency_ms"`
	P50Latency  float64 `json:"p50_latency_ms"`
	P95Latency  float64 `json:"p95_latency_ms"`
	P99Latency  float64 `json:"p99_latency_ms"`
	P999Latency float64 `json:"p999_latency_ms"`
	MinLatency  float64 `json:"min_latency_ms"`
	MaxLatency  float64 `json:"max_latency_ms"`

	// Resource usage
	MemoryHeapMB    float64 `json:"memory_heap_mb"`
	MemorySysMB     float64 `json:"memory_sys_mb"`
	MemoryTotalMB   float64 `json:"memory_total_alloc_mb"`
	Goroutines      int     `json:"goroutines"`
	GCPauseMs       float64 `json:"gc_pause_ms"`
	NumGC           uint32  `json:"num_gc"`

	// Service health
	UptimeSeconds int64 `json:"uptime_seconds"`

	// Network/Proxy metrics
	AvgProxyHops     float64 `json:"avg_proxy_hops"`
	ProxyDetected    bool    `json:"proxy_detected"`
	IstioSidecar     bool    `json:"istio_sidecar_detected"`
	RequestsViaProxy int64   `json:"requests_via_proxy"`

	// System information
	Hostname         string  `json:"hostname"`
	OS               string  `json:"os"`
	Architecture     string  `json:"architecture"`
	NumCPU           int     `json:"num_cpu"`
	KernelVersion    string  `json:"kernel_version"`
	TotalMemoryMB    float64 `json:"total_memory_mb"`
	AvailableMemoryMB float64 `json:"available_memory_mb"`
	TotalDiskGB      float64 `json:"total_disk_gb"`
	AvailableDiskGB  float64 `json:"available_disk_gb"`
	DiskUsagePercent float64 `json:"disk_usage_percent"`
}

var (
	mu               sync.RWMutex
	latencies        []float64
	proxyHops        []int
	requests         atomic.Int64
	errors           atomic.Int64
	requestsViaProxy atomic.Int64
	startTime        time.Time
)

func handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Detect proxy hops from headers
	hops := countProxyHops(r)

	// Simulate some work
	time.Sleep(20 * time.Millisecond)

	lat := float64(time.Since(start).Milliseconds())

	requests.Add(1)
	if hops > 0 {
		requestsViaProxy.Add(1)
	}

	mu.Lock()
	latencies = append(latencies, lat)
	proxyHops = append(proxyHops, hops)
	if len(latencies) > 1000 {
		latencies = latencies[1:]
		proxyHops = proxyHops[1:]
	}
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func countProxyHops(r *http.Request) int {
	hops := 0

	// Check X-Forwarded-For header (counts IPs in chain)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Count commas + 1 for number of IPs
		count := 1
		for _, c := range xff {
			if c == ',' {
				count++
			}
		}
		hops += count
	}

	// Check Via header (standard proxy header)
	if via := r.Header.Get("Via"); via != "" {
		count := 1
		for _, c := range via {
			if c == ',' {
				count++
			}
		}
		hops += count
	}

	// Check Envoy-specific headers (Istio uses Envoy)
	if r.Header.Get("X-Envoy-External-Address") != "" {
		hops++
	}
	if r.Header.Get("X-Envoy-Decorator-Operation") != "" {
		hops++
	}

	// Check for Istio-specific headers
	if r.Header.Get("X-B3-TraceId") != "" {
		// Istio uses B3 propagation
		hops++
	}

	return hops
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	// Copy data under read lock to minimize critical section
	mu.RLock()
	latenciesCopy := make([]float64, len(latencies))
	proxyHopsCopy := make([]int, len(proxyHops))
	copy(latenciesCopy, latencies)
	copy(proxyHopsCopy, proxyHops)
	mu.RUnlock()

	// Get current request counts
	totalRequests := requests.Load()
	totalErrors := errors.Load()
	totalViaProxy := requestsViaProxy.Load()

	// Calculate uptime
	uptime := time.Since(startTime).Seconds()

	// Get runtime memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Detect proxy from current request headers
	currentHops := countProxyHops(r)
	proxyDetected := currentHops > 0
	istioDetected := r.Header.Get("X-B3-TraceId") != "" || r.Header.Get("X-Envoy-Decorator-Operation") != ""

	// Calculate average proxy hops
	avgHops := 0.0
	if len(proxyHopsCopy) > 0 {
		totalHops := 0
		for _, h := range proxyHopsCopy {
			totalHops += h
		}
		avgHops = round(float64(totalHops) / float64(len(proxyHopsCopy)))
	}

	// Get system information
	hostname, kernelVersion := getSystemInfo()
	totalMemMB := getTotalMemoryMB()
	availMemMB := getAvailableMemoryMB()
	totalDiskGB, availDiskGB, diskUsagePercent := getDiskStats()

	if len(latenciesCopy) == 0 {
		stats := Stats{
			Requests:          totalRequests,
			Errors:            totalErrors,
			RequestsPerSecond: round(float64(totalRequests) / uptime),
			SuccessRate:       100.0,
			MemoryHeapMB:      round(float64(memStats.Alloc) / 1024 / 1024),
			MemorySysMB:       round(float64(memStats.Sys) / 1024 / 1024),
			MemoryTotalMB:     round(float64(memStats.TotalAlloc) / 1024 / 1024),
			Goroutines:        runtime.NumGoroutine(),
			GCPauseMs:         round(float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e6),
			NumGC:             memStats.NumGC,
			UptimeSeconds:     int64(uptime),
			AvgProxyHops:      avgHops,
			ProxyDetected:     proxyDetected,
			IstioSidecar:      istioDetected,
			RequestsViaProxy:  totalViaProxy,
			Hostname:          hostname,
			OS:                runtime.GOOS,
			Architecture:      runtime.GOARCH,
			NumCPU:            runtime.NumCPU(),
			KernelVersion:     kernelVersion,
			TotalMemoryMB:     totalMemMB,
			AvailableMemoryMB: availMemMB,
			TotalDiskGB:       totalDiskGB,
			AvailableDiskGB:   availDiskGB,
			DiskUsagePercent:  diskUsagePercent,
		}
		if totalRequests > 0 {
			stats.SuccessRate = round(float64(totalRequests-totalErrors) / float64(totalRequests) * 100)
		}
		json.NewEncoder(w).Encode(stats)
		return
	}

	// Calculate latency statistics
	sum := 0.0
	minLat := latenciesCopy[0]
	maxLat := latenciesCopy[0]

	for _, l := range latenciesCopy {
		sum += l
		if l < minLat {
			minLat = l
		}
		if l > maxLat {
			maxLat = l
		}
	}

	// Calculate percentiles
	p50 := percentile(latenciesCopy, 0.50)
	p95 := percentile(latenciesCopy, 0.95)
	p99 := percentile(latenciesCopy, 0.99)
	p999 := percentile(latenciesCopy, 0.999)

	// Calculate success rate
	successRate := 100.0
	if totalRequests > 0 {
		successRate = float64(totalRequests-totalErrors) / float64(totalRequests) * 100
	}

	stats := Stats{
		// Request metrics
		Requests:          totalRequests,
		Errors:            totalErrors,
		RequestsPerSecond: round(float64(totalRequests) / uptime),
		SuccessRate:       round(successRate),

		// Latency metrics
		AvgLatency:  round(sum / float64(len(latenciesCopy))),
		P50Latency:  round(p50),
		P95Latency:  round(p95),
		P99Latency:  round(p99),
		P999Latency: round(p999),
		MinLatency:  round(minLat),
		MaxLatency:  round(maxLat),

		// Resource usage
		MemoryHeapMB:  round(float64(memStats.Alloc) / 1024 / 1024),
		MemorySysMB:   round(float64(memStats.Sys) / 1024 / 1024),
		MemoryTotalMB: round(float64(memStats.TotalAlloc) / 1024 / 1024),
		Goroutines:    runtime.NumGoroutine(),
		GCPauseMs:     round(float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e6),
		NumGC:         memStats.NumGC,

		// Service health
		UptimeSeconds: int64(uptime),

		// Network/Proxy metrics
		AvgProxyHops:     avgHops,
		ProxyDetected:    proxyDetected,
		IstioSidecar:     istioDetected,
		RequestsViaProxy: totalViaProxy,

		// System information
		Hostname:          hostname,
		OS:                runtime.GOOS,
		Architecture:      runtime.GOARCH,
		NumCPU:            runtime.NumCPU(),
		KernelVersion:     kernelVersion,
		TotalMemoryMB:     totalMemMB,
		AvailableMemoryMB: availMemMB,
		TotalDiskGB:       totalDiskGB,
		AvailableDiskGB:   availDiskGB,
		DiskUsagePercent:  diskUsagePercent,
	}

	json.NewEncoder(w).Encode(stats)
}

func percentile(data []float64, p float64) float64 {
	copyData := append([]float64{}, data...)
	sort.Float64s(copyData)

	idx := int(math.Ceil(p*float64(len(copyData)))) - 1
	return copyData[idx]
}

func round(val float64) float64 {
	return math.Round(val*100) / 100
}

// getSystemInfo collects system information
func getSystemInfo() (hostname, kernelVersion string) {
	// Get hostname
	hostname, _ = os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Get kernel version from uname -a
	if cmd := exec.Command("uname", "-a"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			kernelVersion = strings.TrimSpace(string(output))
		}
	}

	if kernelVersion == "" {
		kernelVersion = "unknown"
	}

	return
}

// getTotalMemoryMB reads total system memory from /proc/meminfo (Linux)
func getTotalMemoryMB() float64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseFloat(fields[1], 64)
				if err == nil {
					return round(kb / 1024) // Convert KB to MB
				}
			}
		}
	}
	return 0
}

// getAvailableMemoryMB reads available system memory from /proc/meminfo (Linux)
func getAvailableMemoryMB() float64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseFloat(fields[1], 64)
				if err == nil {
					return round(kb / 1024) // Convert KB to MB
				}
			}
		}
	}
	return 0
}

// getDiskStats gets filesystem statistics for the root partition
func getDiskStats() (totalGB, availableGB, usagePercent float64) {
	var stat syscall.Statfs_t
	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, 0, 0
	}

	// Calculate total and available space
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	availBytes := stat.Bavail * uint64(stat.Bsize)

	totalGB = round(float64(totalBytes) / 1024 / 1024 / 1024)
	availableGB = round(float64(availBytes) / 1024 / 1024 / 1024)

	if totalGB > 0 {
		usagePercent = round((float64(totalBytes-availBytes) / float64(totalBytes)) * 100)
	}

	return
}

func main() {
	// Initialize start time for uptime tracking
	startTime = time.Now()

	// Pre-allocate slices with capacity
	latencies = make([]float64, 0, 1000)
	proxyHops = make([]int, 0, 1000)

	http.HandleFunc("/", handler)
	http.HandleFunc("/stats", statsHandler)

	log.Println("App running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}