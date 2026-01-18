# PodMeter

A lightweight Go application designed to measure and compare performance metrics between **Istio Ambient Mode** and **Sidecar Mode** deployments.

## Overview

PodMeter provides comprehensive metrics for monitoring pod performance, resource usage, latency, and network proxy behavior. It's specifically built to help measure the performance impact of different Istio service mesh deployment modes.

## Key Features

- **Performance Metrics**: Request throughput, latency percentiles (p50, p95, p99, p999)
- **Resource Monitoring**: Memory usage, goroutines, GC statistics
- **Proxy Detection**: Automatic detection of Istio sidecar vs ambient mode
- **Hop Tracking**: Measures network hops through service mesh proxies
- **Zero Dependencies**: Pure Go stdlib implementation
- **Optimized**: Uses atomic operations, RWMutex, and efficient sorting

## Metrics Exposed

### Request Metrics
- `requests` - Total number of requests processed
- `errors` - Total number of failed requests
- `requests_per_second` - Current throughput
- `success_rate_percent` - Percentage of successful requests

### Latency Distribution
- `avg_latency_ms` - Average response latency
- `p50_latency_ms` - Median latency
- `p95_latency_ms` - 95th percentile latency
- `p99_latency_ms` - 99th percentile latency
- `p999_latency_ms` - 99.9th percentile latency
- `min_latency_ms` / `max_latency_ms` - Min/max response times

### Resource Usage (Critical for Istio Comparison)
- `memory_heap_mb` - Active heap memory
- **`memory_sys_mb`** - Total OS memory (what Kubernetes sees)
- `memory_total_alloc_mb` - Cumulative allocations
- `goroutines` - Number of active goroutines
- `gc_pause_ms` - Latest garbage collection pause time
- `num_gc` - Number of GC cycles

### Network/Proxy Metrics
- **`avg_proxy_hops`** - Average number of proxy hops detected
- `proxy_detected` - Boolean indicating proxy presence
- `istio_sidecar_detected` - Boolean indicating Istio/Envoy detection
- `requests_via_proxy` - Count of requests through proxies

### Service Health
- `uptime_seconds` - Service uptime in seconds

## Quick Start

### Prerequisites
- Go 1.25+ (if building locally)
- Docker (for containerization)
- Kubernetes cluster with Istio (for deployment testing)

### Local Development

```bash
# Build the application
go build -o podmeter main.go

# Run locally
./podmeter

# Test the application
curl http://localhost:8080/

# View metrics
curl http://localhost:8080/stats | jq
```

## Containerization

### Build Docker Image

The project includes a multi-stage Dockerfile for optimal image size:

```bash
# Build the image
docker build -t podmeter:latest .

# Run the container
docker run -p 8080:8080 podmeter:latest

# Test the containerized app
curl http://localhost:8080/stats | jq
```

### Dockerfile Structure

```dockerfile
# Stage 1: Build
FROM golang:1.25-alpine AS build
WORKDIR /app
COPY main.go .
RUN go build -o podmeter main.go

# Stage 2: Runtime
FROM alpine:3.19
WORKDIR /app
COPY --from=build /app/podmeter .
EXPOSE 8080
CMD ["./podmeter"]
```

**Benefits of multi-stage build:**
- Small final image size (~10MB vs ~300MB)
- No build tools in production image
- Improved security posture

## Kubernetes Deployment

### Basic Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: podmeter
  labels:
    app: podmeter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: podmeter
  template:
    metadata:
      labels:
        app: podmeter
    spec:
      containers:
      - name: podmeter
        image: podmeter:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "32Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "200m"
---
apiVersion: v1
kind: Service
metadata:
  name: podmeter
spec:
  selector:
    app: podmeter
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP
```

### Deploy to Kubernetes

```bash
# Apply the deployment
kubectl apply -f deployment.yaml

# Check the pod status
kubectl get pods -l app=podmeter

# Port-forward to access locally
kubectl port-forward svc/podmeter 8080:8080

# View metrics
curl http://localhost:8080/stats | jq
```

## Comparing Istio Modes

### Deploying with Istio Sidecar

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: podmeter-sidecar
  labels:
    app: podmeter
    mode: sidecar
spec:
  replicas: 1
  selector:
    matchLabels:
      app: podmeter
      mode: sidecar
  template:
    metadata:
      labels:
        app: podmeter
        mode: sidecar
      annotations:
        sidecar.istio.io/inject: "true"  # Enable sidecar injection
    spec:
      containers:
      - name: podmeter
        image: podmeter:latest
        ports:
        - containerPort: 8080
```

### Deploying with Istio Ambient Mode

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: podmeter-ambient
  labels:
    app: podmeter
    mode: ambient
spec:
  replicas: 1
  selector:
    matchLabels:
      app: podmeter
      mode: ambient
  template:
    metadata:
      labels:
        app: podmeter
        mode: ambient
        istio.io/dataplane-mode: ambient  # Enable ambient mode
    spec:
      containers:
      - name: podmeter
        image: podmeter:latest
        ports:
        - containerPort: 8080
```

### Key Metrics to Compare

When comparing Istio modes, focus on these metrics:

#### 1. Memory Usage (`memory_sys_mb`) - MOST IMPORTANT
- **Sidecar Mode**: 60-100 MB (app + Envoy proxy)
- **Ambient Mode**: 8-15 MB (just your app)
- Shows the biggest resource difference

#### 2. Latency Percentiles
- **Sidecar Mode**: Higher due to localhost proxy hop
- **Ambient Mode**: Lower, direct routing through node-level ztunnel
- Compare: `p50_latency_ms`, `p95_latency_ms`, `p99_latency_ms`

#### 3. Proxy Hops (`avg_proxy_hops`)
- **Sidecar Mode**: 2-3 hops (client sidecar → server sidecar → app)
- **Ambient Mode**: 1 hop (ztunnel → app)

#### 4. Throughput (`requests_per_second`)
- **Sidecar Mode**: Slightly lower due to proxy overhead
- **Ambient Mode**: Potentially higher

### Example Comparison

**Sidecar Mode Response:**
```json
{
  "memory_sys_mb": 85.5,
  "p50_latency_ms": 24,
  "p99_latency_ms": 38,
  "avg_proxy_hops": 2,
  "istio_sidecar_detected": true,
  "goroutines": 12
}
```

**Ambient Mode Response:**
```json
{
  "memory_sys_mb": 10.2,
  "p50_latency_ms": 21,
  "p99_latency_ms": 28,
  "avg_proxy_hops": 1,
  "istio_sidecar_detected": true,
  "goroutines": 5
}
```

## Load Testing

To generate meaningful metrics, use a load testing tool:

```bash
# Using Apache Bench
ab -n 10000 -c 100 http://localhost:8080/

# Using hey
hey -n 10000 -c 100 http://localhost:8080/

# View the results
curl http://localhost:8080/stats | jq
```

## API Endpoints

### `GET /`
Health check endpoint. Returns "OK" with 20ms simulated processing time.

**Response:**
```
OK
```

### `GET /stats`
Returns JSON with all collected metrics.

**Response:**
```json
{
  "requests": 1000,
  "errors": 0,
  "requests_per_second": 125.5,
  "success_rate_percent": 100,
  "avg_latency_ms": 20.5,
  "p50_latency_ms": 20,
  "p95_latency_ms": 22,
  "p99_latency_ms": 25,
  "p999_latency_ms": 30,
  "min_latency_ms": 20,
  "max_latency_ms": 45,
  "memory_heap_mb": 2.5,
  "memory_sys_mb": 12.8,
  "memory_total_alloc_mb": 15.3,
  "goroutines": 6,
  "gc_pause_ms": 0.05,
  "num_gc": 3,
  "uptime_seconds": 120,
  "avg_proxy_hops": 0,
  "proxy_detected": false,
  "istio_sidecar_detected": false,
  "requests_via_proxy": 0
}
```

## Architecture

### Performance Optimizations

1. **Efficient Sorting**: Uses Go's `sort.Float64s()` (O(n log n)) instead of bubble sort
2. **Read/Write Mutex**: `sync.RWMutex` allows concurrent reads in stats handler
3. **Atomic Operations**: Lock-free counters for request/error tracking
4. **Minimal Critical Sections**: Data copied under lock, calculations done outside
5. **Pre-allocated Slices**: Capacity set to 1000 to reduce allocations

### How Proxy Detection Works

The application inspects HTTP headers injected by Istio/Envoy:
- `X-Forwarded-For` - Proxy chain IPs
- `Via` - Standard HTTP proxy header
- `X-Envoy-External-Address` - Envoy proxy marker
- `X-Envoy-Decorator-Operation` - Envoy operation metadata
- `X-B3-TraceId` - Istio distributed tracing header (B3 propagation)

## Contributing

Contributions are welcome! Areas for improvement:
- Additional metrics (CPU usage, network I/O)
- Prometheus metrics export
- Custom percentile calculations
- Histogram support

## License

See LICENSE file for details.

## Use Cases

- **Istio Migration Planning**: Measure before/after impact of switching to ambient mode
- **Resource Optimization**: Identify memory savings from ambient mode
- **Performance Benchmarking**: Compare latency between deployment modes
- **Cost Analysis**: Calculate infrastructure savings from reduced sidecar overhead
- **SRE Monitoring**: Track service health and performance metrics

## Troubleshooting

### Metrics show 0 proxy hops but Istio is installed
- Ensure traffic is flowing through the mesh
- Check if mTLS is properly configured
- Verify Istio labels are correctly applied

### High memory usage in ambient mode
- Check for memory leaks with `num_gc` and `gc_pause_ms`
- Verify the pod isn't being injected with sidecars accidentally
- Review Kubernetes resource limits

### Latency not showing improvement in ambient mode
- Verify ambient mode is properly enabled
- Check ztunnel deployment status
- Review network policies that might add overhead

## Additional Resources

- [Istio Ambient Mesh Documentation](https://istio.io/latest/docs/ambient/)
- [Istio Sidecar vs Ambient Comparison](https://istio.io/latest/docs/ambient/overview/)
- [Kubernetes Resource Management](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
