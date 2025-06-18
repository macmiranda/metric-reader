# Metric Reader

A program that monitors Prometheus metrics and executes actions when thresholds are exceeded for a specified duration. Due to its small footprint, it was designed to run as a sidecar container in a Kubernetes Pod, though it can also be run as a service, and with the right plugin it can interact with other services and cloud provider APIs.

## Features

- Monitor any Prometheus metric
- Configurable thresholds with duration requirements
- Plugin system for custom actions
- Built-in logging and file creation plugins
- Configurable polling interval and backoff periods
- Leader election mechanism for running multiple replicas at the same time with a single action outcome.

## Quick Start with Just

This project uses [Just](https://github.com/casey/just) as a command runner. Install it first, then you can use the following commands:

```bash
# List all available commands
just

# Build the application
just build

# Build plugin .so files
just build-plugins

# Run all tests
just run-tests

# Build Docker image
just build-image

# Start services using Docker Compose
just compose-up

# Stop and remove services using Docker Compose
just compose-down

# Create and configure Kind cluster
just kind-up

# Delete Kind cluster
just kind-down

# Deploy metric-reader to Kind cluster
just k8s-apply

# Delete metric-reader from Kind cluster
just k8s-delete
```

## Configuration

The service is configured through environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `METRIC_NAME` | Name of the Prometheus metric to monitor | (required) |
| `LABEL_FILTERS` | Label filters to apply to the metric query | (optional) |
| `THRESHOLD` | Threshold value with operator (e.g., ">100" or "<50") | (required) |
| `THRESHOLD_DURATION` | How long the threshold must be exceeded before action | 0s |
| `POLLING_INTERVAL` | How often to check the metric | 1s |
| `BACKOFF_DELAY` | Delay between actions after threshold is triggered | 0s |
| `PROMETHEUS_ENDPOINT` | Prometheus server URL | http://prometheus:9090 |
| `PLUGIN_DIR` | Directory containing plugin .so files | (optional) |
| `ACTION_PLUGIN` | Name of the plugin to use for actions | (optional) |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | info |
| `LEADER_ELECTION_ENABLED` | Whether to enable leader election | true |
| `LEADER_ELECTION_LOCK_NAME` | Name of the lock to use for leader election | metric-reader-leader |

## Available Plugins

### File Action Plugin

Creates a file of configurable size when a metric threshold is exceeded.

**Configuration:**

- `FILE_ACTION_DIR`: Directory where files will be created (default: `/tmp/metric-files`)
- `FILE_ACTION_SIZE`: Size of files to create in bytes (default: 1MB)

### Log Action Plugin

Logs threshold events with detailed information about the metric value and duration.

## Building

```bash
# Build the main service
go build -o metric-reader

# Build plugins (from the metric-reader directory)
go build -buildmode=plugin -o plugins/file_action.so plugins/file_action/file_action.go
go build -buildmode=plugin -o plugins/log_action.so plugins/log_action/log_action.go
```

## Docker

Build and run using Docker:

```bash
# Build the image
docker build -t metric-reader .

# Run the container
docker run -d \
  -e METRIC_NAME="your_metric" \
  -e THRESHOLD=">100" \
  -e THRESHOLD_DURATION="5m" \
  -e ACTION_PLUGIN="file_action" \
  -e PLUGIN_DIR="/plugins" \
  -v /path/to/plugins:/plugins \
  metric-reader
```

## Example Use Cases

### AWS EFS Burst Credit Monitoring

Monitor AWS EFS burst credits and automatically generate I/O activity to increase credits when they fall below a threshold.

## Creating Custom Plugins

See the [plugins README](plugins/README.md) for information on creating custom plugins.
