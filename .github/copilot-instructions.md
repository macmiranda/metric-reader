# Copilot Instructions for metric-reader

## Project Overview

Metric-reader is a lightweight Go application that monitors Prometheus metrics and executes configurable actions when thresholds are exceeded for a specified duration. It's designed to run as a sidecar container in Kubernetes Pods but can also run as a standalone service.

### Key Capabilities
- Monitor any Prometheus metric with configurable thresholds
- Execute custom actions via a plugin system
- Support for leader election when running multiple replicas
- Configurable polling intervals and backoff periods
- Built-in logging and file creation plugins

## Architecture

### Core Components

1. **Main Application** (`main.go`)
   - Prometheus metric polling and threshold checking
   - Environment-based configuration
   - Plugin execution coordination
   - Uses `github.com/rs/zerolog` for structured logging

2. **Plugin System** (`plugin.go`)
   - Dynamic plugin loading from `.so` files
   - Plugin registry for managing available plugins
   - `ActionPlugin` interface that all plugins must implement

3. **Leader Election** (`leader_election.go`)
   - Kubernetes-based leader election using coordination leases
   - Prevents duplicate actions when running multiple replicas
   - Automatic fallback to single-instance mode outside Kubernetes

4. **Built-in Plugins**
   - `file_action`: Creates files of configurable size
   - `log_action`: Logs threshold events with detailed information

## Build and Test

### Prerequisites
- Go 1.21 or later
- [Just](https://github.com/casey/just) command runner (optional but recommended)

### Build Commands

```bash
# Using Just (recommended)
just build              # Build the main application
just build-plugins      # Build plugin .so files
just run-tests         # Run all tests

# Using go commands directly
go build -o metric-reader .
go build -buildmode=plugin -o plugins/file_action.so plugins/file_action/file_action.go
go build -buildmode=plugin -o plugins/log_action.so plugins/log_action/log_action.go
go test -v ./...
```

### Docker

```bash
just build-image       # Build Docker image
just compose-up        # Start services with Docker Compose
just compose-down      # Stop Docker Compose services
```

### Kubernetes

```bash
just kind-up           # Create Kind cluster
just k8s-apply         # Deploy to cluster
just k8s-delete        # Remove from cluster
just kind-down         # Delete Kind cluster
```

## Code Conventions

### General Guidelines
- Follow standard Go conventions and idioms
- Use `zerolog` for all logging (already imported as `log`)
- Handle errors explicitly; avoid ignoring errors
- Use context for cancellation and timeouts
- Prefer descriptive variable names over abbreviations

### Logging Standards
- Use structured logging with `zerolog`
- Log levels:
  - `Debug`: Detailed diagnostic information
  - `Info`: Normal operational messages
  - `Warn`: Warning conditions that should be reviewed
  - `Error`: Error conditions that prevent normal operation
  - `Fatal`: Critical errors that require program termination

Example:
```go
log.Info().
    Str("metric_name", metricName).
    Float64("value", value).
    Msg("processing metric")
```

### Error Handling
- Return errors with context using `fmt.Errorf("description: %v", err)`
- Log errors before returning them when appropriate
- Use `log.Fatal()` only for startup configuration errors

## Plugin Development

### Plugin Interface

All plugins must implement:

```go
type ActionPlugin interface {
    Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error
    Name() string
}
```

### Plugin Structure

1. Package must be `main`
2. Implement the `ActionPlugin` interface
3. Export a variable named `Plugin` of your plugin type
4. Use `init()` for configuration and setup
5. Build with `-buildmode=plugin`

### Plugin Configuration
- Use environment variables for plugin settings
- Prefix variables with plugin name (e.g., `FILE_ACTION_DIR`)
- Provide sensible defaults
- Document all configuration options

### Plugin Best Practices
- Use the provided context for cancellation
- Return meaningful errors from `Execute()`
- Use `zerolog` for consistent logging
- Test plugins thoroughly before deployment
- Handle missing configuration gracefully

## Environment Variables

### Required Variables
- `METRIC_NAME`: Prometheus metric to monitor

### Optional Variables
- `LABEL_FILTERS`: Label filters for metric query (e.g., `job="node-exporter"`)
- `THRESHOLD`: Threshold with operator (e.g., `>100`, `<50`)
- `THRESHOLD_DURATION`: Duration threshold must be exceeded (default: `0s`)
- `POLLING_INTERVAL`: Metric check frequency (default: `1s`)
- `BACKOFF_DELAY`: Delay between actions (default: `0s`)
- `PROMETHEUS_ENDPOINT`: Prometheus URL (default: `http://prometheus:9090`)
- `PLUGIN_DIR`: Directory containing plugin `.so` files
- `ACTION_PLUGIN`: Plugin name to execute
- `LOG_LEVEL`: Logging level - `debug`, `info`, `warn`, `error` (default: `info`)
- `LEADER_ELECTION_ENABLED`: Enable leader election (default: `true`)
- `LEADER_ELECTION_LOCK_NAME`: Lock name for leader election (default: `metric-reader-leader`)
- `POD_NAMESPACE`: Kubernetes namespace (injected via Downward API)

### Plugin-specific Variables

**File Action Plugin:**
- `FILE_ACTION_DIR`: Output directory (default: `/tmp/metric-files`)
- `FILE_ACTION_SIZE`: File size in bytes (default: `1048576` = 1MB)

## Dependencies

Key dependencies:
- `github.com/prometheus/client_golang`: Prometheus client
- `github.com/rs/zerolog`: Structured logging
- `k8s.io/client-go`: Kubernetes client for leader election
- `k8s.io/apimachinery`: Kubernetes API types

Use `go mod` for dependency management:
```bash
go mod download    # Download dependencies
go mod tidy        # Clean up dependencies
go mod verify      # Verify dependencies
```

## Development Workflow

1. **Making Changes**
   - Create a feature branch
   - Make focused, incremental changes
   - Test locally using `just run-tests` or `go test -v ./...`
   - Build to verify compilation

2. **Testing**
   - Currently no unit tests exist (test infrastructure to be added)
   - Test manually using Docker Compose or Kind cluster
   - Verify plugin loading and execution
   - Test leader election behavior with multiple replicas

3. **Code Style**
   - Run `go fmt` before committing
   - Use `go vet` to check for common issues
   - Follow Go standard library patterns

4. **Commits**
   - Write clear, descriptive commit messages
   - Keep commits focused on a single change
   - Reference issue numbers when applicable

## Common Tasks

### Adding a New Plugin

1. Create directory: `plugins/my_plugin/`
2. Implement `ActionPlugin` interface
3. Export `Plugin` variable
4. Add build command to `Justfile`
5. Document configuration in plugin README
6. Update main README with plugin information

### Modifying Metric Query Logic

- All query logic is in `main.go`
- Threshold parsing is in `parseThreshold()` function
- Threshold checking happens in the main polling loop
- Consider leader election when executing actions

### Updating Dependencies

```bash
go get -u github.com/package/name  # Update specific package
go mod tidy                         # Clean up go.mod
go test -v ./...                    # Verify changes
```

## Security Considerations

- Plugins are loaded from trusted directories only
- Use context timeouts for Prometheus queries
- Leader election prevents duplicate actions
- Validate all environment variable inputs
- Handle Kubernetes RBAC for leader election

## Troubleshooting

### Common Issues

1. **Plugin not loading**
   - Verify plugin built with same Go version as main app
   - Check plugin is in correct directory
   - Ensure `PLUGIN_DIR` is set correctly
   - Review logs for plugin loading errors

2. **Leader election not working**
   - Verify in-cluster Kubernetes configuration
   - Check RBAC permissions for lease resources
   - Ensure `POD_NAMESPACE` is set (via Downward API)

3. **Metrics not being read**
   - Verify Prometheus endpoint is accessible
   - Check metric name and label filters
   - Review Prometheus query warnings in logs
   - Increase `LOG_LEVEL=debug` for more details

## Additional Resources

- [Plugin Development Guide](../plugins/README.md)
- [Prometheus Query Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Kubernetes Leader Election](https://pkg.go.dev/k8s.io/client-go/tools/leaderelection)
- [Zerolog Documentation](https://github.com/rs/zerolog)
