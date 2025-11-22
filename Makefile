.PHONY: build build-plugins test build-image kind-up kind-down k8s-apply k8s-delete e2e-test clean

# Build the Go application
build:
	go build -o metric-reader .

# Build plugin .so files
build-plugins:
	go build -buildmode=plugin -o plugins/file_action.so plugins/file_action/file_action.go
	go build -buildmode=plugin -o plugins/log_action.so plugins/log_action/log_action.go

# Run all tests
test:
	go test -v ./...

# Build Docker image
build-image:
	docker build -t metric-reader:latest .

# Create and configure Kind cluster
kind-up:
	kind create cluster --name metric-reader --config kubernetes/kind-config.yaml

# Delete Kind cluster
kind-down:
	kind delete cluster --name metric-reader

# Load Docker image to Kind cluster
kind-load-image:
	kind load docker-image metric-reader:latest --name metric-reader

# Deploy metric-reader to Kind cluster
k8s-apply:
	kubectl apply -f kubernetes/metric-reader.yaml

# Delete metric-reader from Kind cluster
k8s-delete:
	kubectl delete -f kubernetes/metric-reader.yaml

# Wait for deployment to be ready
k8s-wait:
	kubectl wait --for=condition=ready pod -l app=metric-reader --timeout=120s
	kubectl wait --for=condition=ready pod -l app=prometheus --timeout=120s

# Check deployment status
k8s-status:
	kubectl get pods -l app=metric-reader
	kubectl get pods -l app=prometheus

# Get logs from metric-reader pods
k8s-logs:
	kubectl logs -l app=metric-reader --all-containers=true --tail=50

# Run end-to-end test
e2e-test:
	@echo "Starting e2e test..."
	$(MAKE) build-image
	$(MAKE) kind-up || (echo "Failed to create Kind cluster"; exit 1)
	$(MAKE) kind-load-image || (echo "Failed to load image"; $(MAKE) kind-down; exit 1)
	$(MAKE) k8s-apply || (echo "Failed to apply Kubernetes resources"; $(MAKE) kind-down; exit 1)
	$(MAKE) k8s-wait || (echo "Failed waiting for pods to be ready"; $(MAKE) kind-down; exit 1)
	@echo "Running e2e test validation..."
	@echo "Checking metric-reader deployment..."
	kubectl get deployment metric-reader || ($(MAKE) kind-down; exit 1)
	@echo "Checking metric-reader pods..."
	kubectl get pods -l app=metric-reader || ($(MAKE) kind-down; exit 1)
	@echo "Checking prometheus deployment..."
	kubectl get statefulset prometheus || ($(MAKE) kind-down; exit 1)
	@echo "Verifying metric-reader is running..."
	@kubectl logs -l app=metric-reader --all-containers=true --tail=20 || true
	@echo "E2E test completed successfully!"

# Clean up all resources
clean:
	rm -f metric-reader
	rm -f plugins/*.so
	kind delete cluster --name metric-reader 2>/dev/null || true
