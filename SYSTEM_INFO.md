# System Information Metrics

## Overview

PodMeter now includes comprehensive system information that helps you understand the environment where your application is running. This is especially useful for comparing resource usage between Istio sidecar and ambient modes.

## New Metrics Added

### System Information
- **`hostname`** - Pod/container hostname
- **`os`** - Operating system (linux, darwin, etc.)
- **`architecture`** - CPU architecture (amd64, arm64, etc.)
- **`num_cpu`** - Number of CPU cores available
- **`kernel_version`** - Full kernel version from `uname -a`
- **`total_memory_mb`** - Total system memory in MB
- **`available_memory_mb`** - Available system memory in MB
- **`total_disk_gb`** - Total disk space in GB
- **`available_disk_gb`** - Available disk space in GB
- **`disk_usage_percent`** - Disk usage percentage

## Example Output

### Local macOS Development
```json
{
  "hostname": "nyanlintun",
  "os": "darwin",
  "architecture": "arm64",
  "num_cpu": 16,
  "kernel_version": "Darwin nyanlintun 25.1.0 Darwin Kernel Version 25.1.0: Mon Oct 20 19:34:05 PDT 2025; root:xnu-12377.41.6~2/RELEASE_ARM64_T6041 arm64",
  "total_memory_mb": 0,
  "available_memory_mb": 0,
  "total_disk_gb": 1858.19,
  "available_disk_gb": 498.76,
  "disk_usage_percent": 73.16
}
```

Note: Memory metrics show `0` on macOS because `/proc/meminfo` is Linux-specific. This works correctly in containers.

### Linux Container/Kubernetes
```json
{
  "hostname": "podmeter-sidecar-7d8f9b5c6d-xk2pm",
  "os": "linux",
  "architecture": "amd64",
  "num_cpu": 2,
  "kernel_version": "Linux podmeter-sidecar-7d8f9b5c6d-xk2pm 5.15.0-1048-gcp #56-Ubuntu SMP Mon Oct 16 18:58:09 UTC 2023 x86_64 GNU/Linux",
  "total_memory_mb": 2048.0,
  "available_memory_mb": 1456.32,
  "total_disk_gb": 100.0,
  "available_disk_gb": 85.5,
  "disk_usage_percent": 14.5
}
```

## Use Cases for System Information

### 1. Architecture Verification
Verify that your pods are running on the expected architecture:
- `arm64` for ARM-based nodes (Graviton, Apple Silicon)
- `amd64` for x86_64 nodes

### 2. Resource Allocation Comparison
Compare resources between sidecar and ambient modes:

**Sidecar Mode:**
```json
{
  "num_cpu": 2,
  "total_memory_mb": 2048,
  "memory_sys_mb": 85.5,  // App + Envoy using ~85MB
  "memory_heap_mb": 15.2
}
```

**Ambient Mode:**
```json
{
  "num_cpu": 2,
  "total_memory_mb": 2048,
  "memory_sys_mb": 10.2,  // App only using ~10MB
  "memory_heap_mb": 2.5
}
```

### 3. Resource Utilization Percentage
Calculate how much of the available resources your pod is using:

```
Memory Utilization = (memory_sys_mb / total_memory_mb) * 100
CPU Usage = goroutines / num_cpu (rough estimate)
```

### 4. Environment Debugging
Quickly identify:
- What kernel version is running
- What hostname the pod has
- What OS/architecture combination
- Available vs total resources

### 5. Capacity Planning
- See total vs available memory
- Monitor disk usage trends
- Identify resource constraints

## Istio Comparison Example

### Sidecar Mode
```json
{
  "hostname": "podmeter-sidecar-abc123",
  "num_cpu": 2,
  "total_memory_mb": 2048,
  "available_memory_mb": 1500,
  "memory_sys_mb": 95.5,
  "memory_heap_mb": 12.3,
  "goroutines": 15,
  "avg_proxy_hops": 2
}
```

**Analysis:**
- Using 95.5MB out of 2048MB available (4.7%)
- Additional goroutines from Envoy proxy
- 2 proxy hops detected

### Ambient Mode
```json
{
  "hostname": "podmeter-ambient-xyz789",
  "num_cpu": 2,
  "total_memory_mb": 2048,
  "available_memory_mb": 1800,
  "memory_sys_mb": 10.2,
  "memory_heap_mb": 2.8,
  "goroutines": 5,
  "avg_proxy_hops": 1
}
```

**Analysis:**
- Using only 10.2MB out of 2048MB available (0.5%)
- Minimal goroutines (no sidecar)
- 1 proxy hop (ztunnel)
- **85MB memory savings per pod** (95.5 - 10.2)

## Key Comparisons for Istio Modes

| Metric | Sidecar Mode | Ambient Mode | Difference |
|--------|--------------|--------------|------------|
| `memory_sys_mb` | 85-100 MB | 8-15 MB | ~85 MB saved |
| `goroutines` | 12-20 | 3-6 | Fewer threads |
| `avg_proxy_hops` | 2-3 | 1 | Reduced hops |
| Resource % | 4-5% | 0.5-1% | 80% reduction |

## Platform-Specific Notes

### Linux (Container/Kubernetes)
- All metrics fully supported
- Memory info read from `/proc/meminfo`
- Kernel version from `uname -a`

### macOS (Local Development)
- `total_memory_mb` and `available_memory_mb` will be `0`
- All other metrics work correctly
- Use Docker/Kubernetes for full metrics

### Windows
- Limited support (untested)
- Some metrics may not be available

## API Usage

Get system info along with all other metrics:
```bash
curl http://localhost:8080/stats | jq '{
  hostname,
  os,
  architecture,
  num_cpu,
  total_memory_mb,
  available_memory_mb,
  memory_sys_mb,
  disk_usage_percent
}'
```

Compare two deployments:
```bash
# Sidecar
kubectl exec -it deployment/podmeter-sidecar -- wget -qO- localhost:8080/stats | jq '{hostname, num_cpu, total_memory_mb, memory_sys_mb}'

# Ambient
kubectl exec -it deployment/podmeter-ambient -- wget -qO- localhost:8080/stats | jq '{hostname, num_cpu, total_memory_mb, memory_sys_mb}'
```

## Monitoring Dashboard Example

You can use these metrics to create a comparison dashboard:

```
Pod: podmeter-sidecar-abc123
├─ Environment
│  ├─ OS: linux/amd64
│  ├─ CPUs: 2
│  └─ Total Memory: 2048 MB
├─ Resource Usage
│  ├─ Memory: 95.5 MB (4.7%)
│  ├─ Goroutines: 15
│  └─ Disk: 14.5% used
└─ Network
   ├─ Proxy Hops: 2
   └─ Istio: Sidecar

Pod: podmeter-ambient-xyz789
├─ Environment
│  ├─ OS: linux/amd64
│  ├─ CPUs: 2
│  └─ Total Memory: 2048 MB
├─ Resource Usage
│  ├─ Memory: 10.2 MB (0.5%)
│  ├─ Goroutines: 5
│  └─ Disk: 14.5% used
└─ Network
   ├─ Proxy Hops: 1
   └─ Istio: Ambient

Savings: 85.3 MB per pod (89% memory reduction)
```

## Implementation Details

### Memory Information (Linux)
Read from `/proc/meminfo`:
- `MemTotal` → `total_memory_mb`
- `MemAvailable` → `available_memory_mb`

### Disk Information
Uses `syscall.Statfs("/")` to get:
- Total blocks × block size = total disk
- Available blocks × block size = available disk

### CPU Information
Uses `runtime.NumCPU()` to get the number of logical CPU cores.

### Kernel Information
Executes `uname -a` command to get full system information.

## Troubleshooting

### Memory shows 0
- This is expected on macOS
- Will work correctly in Linux containers
- Verify you're running on Linux: check `os` field

### Disk usage seems wrong
- Disk stats are for the root filesystem (`/`)
- In containers, this shows the container's view of storage
- Kubernetes ephemeral storage may differ from host

### num_cpu doesn't match expectations
- Shows logical CPUs (including hyperthreading)
- In Kubernetes, shows CPUs available to the container
- May be limited by CPU resource limits

### Kernel version is "unknown"
- `uname` command may not be available in minimal containers
- Alpine-based images may have limited tools
- Not critical for most use cases
