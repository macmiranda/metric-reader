# Set build platform based on CPU architecture
ARCH := `uname -m`
TARGET_PLATFORM := if ARCH == "x86_64" { "amd64" } else if ARCH == "aarch64" { "arm64" } else { ARCH }

# Default recipe to run when just is called without arguments
default:
    just --list

# Build the Go application
build:
    GOARCH={{TARGET_PLATFORM}} go build -o metric-reader .

# Build plugin .so files
build-plugins:
    GOARCH={{TARGET_PLATFORM}} go build -buildmode=plugin -o plugins/file_action.so plugins/file_action/file_action.go
    GOARCH={{TARGET_PLATFORM}} go build -buildmode=plugin -o plugins/log_action.so plugins/log_action/log_action.go
    GOARCH={{TARGET_PLATFORM}} go build -buildmode=plugin -o plugins/efs_emergency.so plugins/efs_emergency/efs_emergency.go

# Run all tests
run-tests:
    go test -v ./...

# Build Docker image
build-image:
    docker buildx build --platform linux/{{TARGET_PLATFORM}} --network host -t metric-reader:latest .

# Start services using Docker Compose
compose-up:
    docker-compose up -d

# Stop and remove services using Docker Compose
compose-down:
    docker-compose down

# Create and configure Kind cluster
kind-up:
    kind create cluster --name metric-reader --config kubernetes/kind-config.yaml
    just kind-load-image

# Delete Kind cluster
kind-down:
    kind delete cluster --name metric-reader

# Load Docker image to Kind cluster (useful for reloading after rebuilding)
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
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Starting e2e test..."
    just build-image || { echo "Failed to build Docker image"; exit 1; }
    just kind-up || { echo "Failed to create Kind cluster"; just kind-down; exit 1; }
    just k8s-apply || { echo "Failed to apply Kubernetes resources"; just kind-down; exit 1; }
    just k8s-wait || { echo "Failed waiting for pods to be ready"; just kind-down; exit 1; }
    echo "Running e2e test validation..."
    echo "Checking metric-reader deployment..."
    kubectl get deployment metric-reader || { just kind-down; exit 1; }
    echo "Checking metric-reader pods..."
    kubectl get pods -l app=metric-reader || { just kind-down; exit 1; }
    echo "Checking prometheus deployment..."
    kubectl get statefulset prometheus || { just kind-down; exit 1; }
    echo "Verifying metric-reader is running..."
    kubectl logs -l app=metric-reader --all-containers=true --tail=20 || true
    echo "E2E test completed successfully!"

# Clean up all resources
clean:
    rm -f metric-reader
    rm -f plugins/*.so
    kind delete cluster --name metric-reader 2>/dev/null || true
