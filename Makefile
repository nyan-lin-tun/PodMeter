.PHONY: build run test docker-build docker-run k8s-deploy k8s-deploy-sidecar k8s-deploy-ambient clean help

APP_NAME=podmeter
DOCKER_IMAGE=$(APP_NAME):latest
DOCKER_REGISTRY?=docker.io/yourusername

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the Go binary
	@echo "Building $(APP_NAME)..."
	go build -o $(APP_NAME) main.go
	@echo "Build complete!"

run: build ## Build and run the application locally
	@echo "Starting $(APP_NAME) on :8080..."
	./$(APP_NAME)

test: ## Run a quick test of the application
	@echo "Testing the application..."
	@curl -s http://localhost:8080/ || echo "App not running. Start with 'make run' first."
	@echo "\nFetching stats..."
	@curl -s http://localhost:8080/stats | jq || echo "App not running or jq not installed"

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE)..."
	docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image built successfully!"

docker-run: docker-build ## Build and run Docker container
	@echo "Running Docker container..."
	docker run -p 8080:8080 --name $(APP_NAME) --rm $(DOCKER_IMAGE)

docker-push: docker-build ## Build and push Docker image to registry
	@echo "Tagging image for registry..."
	docker tag $(DOCKER_IMAGE) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)
	@echo "Pushing to $(DOCKER_REGISTRY)..."
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE)

k8s-deploy: ## Deploy basic version to Kubernetes
	@echo "Deploying to Kubernetes..."
	kubectl apply -f deployment.yaml
	@echo "Deployment complete!"
	@echo "Run 'kubectl port-forward svc/podmeter 8080:8080' to access"

k8s-deploy-sidecar: ## Deploy Istio sidecar version
	@echo "Deploying Istio sidecar version..."
	kubectl apply -f deployment-sidecar.yaml
	@echo "Sidecar deployment complete!"
	@echo "Run 'kubectl port-forward svc/podmeter-sidecar 8080:8080' to access"

k8s-deploy-ambient: ## Deploy Istio ambient version
	@echo "Deploying Istio ambient version..."
	kubectl apply -f deployment-ambient.yaml
	@echo "Ambient deployment complete!"
	@echo "Run 'kubectl port-forward svc/podmeter-ambient 8080:8080' to access"

k8s-delete: ## Delete all Kubernetes deployments
	@echo "Deleting Kubernetes resources..."
	-kubectl delete -f deployment.yaml
	-kubectl delete -f deployment-sidecar.yaml
	-kubectl delete -f deployment-ambient.yaml
	@echo "Cleanup complete!"

k8s-logs-sidecar: ## Show logs from sidecar deployment
	kubectl logs -l app=podmeter,mode=sidecar -f

k8s-logs-ambient: ## Show logs from ambient deployment
	kubectl logs -l app=podmeter,mode=ambient -f

k8s-stats-sidecar: ## Get stats from sidecar deployment
	@echo "Fetching stats from sidecar deployment..."
	@kubectl exec -it deployment/podmeter-sidecar -- wget -qO- localhost:8080/stats | jq

k8s-stats-ambient: ## Get stats from ambient deployment
	@echo "Fetching stats from ambient deployment..."
	@kubectl exec -it deployment/podmeter-ambient -- wget -qO- localhost:8080/stats | jq

compare: ## Compare sidecar vs ambient metrics (requires both deployed)
	@echo "=== SIDECAR MODE ==="
	@kubectl exec -it deployment/podmeter-sidecar -- wget -qO- localhost:8080/stats 2>/dev/null | jq '{memory_sys_mb, p50_latency_ms, p99_latency_ms, avg_proxy_hops, istio_sidecar_detected, goroutines}'
	@echo "\n=== AMBIENT MODE ==="
	@kubectl exec -it deployment/podmeter-ambient -- wget -qO- localhost:8080/stats 2>/dev/null | jq '{memory_sys_mb, p50_latency_ms, p99_latency_ms, avg_proxy_hops, istio_sidecar_detected, goroutines}'

clean: ## Clean build artifacts
	@echo "Cleaning up..."
	rm -f $(APP_NAME)
	-docker rm -f $(APP_NAME) 2>/dev/null || true
	@echo "Cleanup complete!"

load-test: ## Run a simple load test with curl
	@echo "Running load test (1000 requests)..."
	@for i in $$(seq 1 1000); do curl -s http://localhost:8080/ > /dev/null; done
	@echo "Load test complete! Check stats:"
	@curl -s http://localhost:8080/stats | jq
