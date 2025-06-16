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

# Delete Kind cluster
kind-down:
    kind delete cluster --name metric-reader

# Deploy metric-reader to Kind cluster
k8s-apply:
    kubectl apply -f kubernetes/metric-reader.yaml

# Delete metric-reader from Kind cluster
k8s-delete:
    kubectl delete -f kubernetes/metric-reader.yaml
