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

# Run end-to-end tests (creates Kind cluster, deploys app, and validates)
just e2e-test

# Clean up (remove binaries, plugins, and Kind cluster)
just clean
```

## Configuration

The service can be configured through either a configuration file or environment variables. Environment variables take precedence over configuration file values.

### Configuration File

Create a `config.toml` file in the current directory or `/etc/metric-reader/` with your configuration. See `config.toml.example` for a complete example.

Example `config.toml`:

```toml
log_level = "info"
metric_name = "up"
threshold = ">0"
threshold_duration = "30s"
polling_interval = "15s"
prometheus_endpoint = "http://prometheus:9090"
```

### Environment Variables

All configuration options can be set via environment variables using uppercase names:

| Variable | Description | Default |
|----------|-------------|---------|
| `METRIC_NAME` | Name of the Prometheus metric to monitor | (required) |
| `LABEL_FILTERS` | Label filters to apply to the metric query | (optional) |
| `THRESHOLD_OPERATOR` | Threshold operator: `greater_than` or `less_than` | (required with thresholds) |
| `SOFT_THRESHOLD` | Soft threshold value (float) | (optional) |
| `HARD_THRESHOLD` | Hard threshold value (float) | (optional) |
| `SOFT_THRESHOLD_PLUGIN` | Plugin to execute when soft threshold is exceeded | (optional) |
| `HARD_THRESHOLD_PLUGIN` | Plugin to execute when hard threshold is exceeded | (optional) |
| `THRESHOLD_DURATION` | How long the threshold must be exceeded before action | 0s |
| `POLLING_INTERVAL` | How often to check the metric | 1s |
| `BACKOFF_DELAY` | Delay between actions after threshold is triggered | 0s |
| `PROMETHEUS_ENDPOINT` | Prometheus server URL | http://prometheus:9090 |
| `PLUGIN_DIR` | Directory containing plugin .so files | (optional) |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | info |
| `LEADER_ELECTION_ENABLED` | Whether to enable leader election | true |
| `LEADER_ELECTION_LOCK_NAME` | Name of the lock to use for leader election | metric-reader-leader |
| `LEADER_ELECTION_LOCK_NAMESPACE` | Kubernetes namespace for leader election lock (uses pod's namespace if not set) | (optional) |
| `POD_NAMESPACE` | Kubernetes namespace for leader election | default |
| `MISSING_VALUE_BEHAVIOR` | Behavior when metric returns no data: `last_value`, `zero`, `assume_breached` | zero |

### Missing Value Behavior

When a Prometheus query returns no data, the behavior is controlled by `MISSING_VALUE_BEHAVIOR`:

- **`last_value`**: Uses the last successfully retrieved metric value. If no previous value exists, threshold checks are skipped for that iteration.
- **`zero`** (default): Treats the missing value as 0 and processes threshold checks normally.
- **`assume_breached`**: Immediately marks all configured thresholds as crossed, starting the threshold duration timer. This is useful for detecting when a metric disappears entirely.

## Available Plugins

### File Action Plugin

Creates a file of configurable size when a metric threshold is exceeded.

**Configuration:**

- `FILE_ACTION_DIR`: Directory where files will be created (default: `/tmp/metric-files`)
- `FILE_ACTION_SIZE`: Size of files to create in bytes (default: 1MB)

### Log Action Plugin

Logs threshold events with detailed information about the metric value and duration.

### EFS Emergency Plugin

Switches an AWS EFS filesystem from bursting throughput mode to elastic throughput mode. Designed for emergency situations when EFS burst credits are depleted.

**Configuration (via config file or environment variables):**

- `efs_file_system_id` / `EFS_FILE_SYSTEM_ID`: The EFS filesystem ID (static - optional if using label)
- `efs_file_system_prometheus_label` / `EFS_FILE_SYSTEM_PROMETHEUS_LABEL`: Prometheus metric label name to extract filesystem ID from (optional if using static ID)
- `aws_region` / `AWS_REGION`: AWS region where the filesystem is located (optional, auto-detected)
- `prometheus_endpoint` / `PROMETHEUS_ENDPOINT`: Prometheus server URL (optional, default: `http://prometheus:9090`)

**Requirements:**

- AWS credentials (supports IRSA on EKS, EC2 instance profiles, environment variables)
- IAM permissions: `elasticfilesystem:UpdateFileSystem`, `elasticfilesystem:DescribeFileSystems`

See [plugins/efs_emergency/README.md](plugins/efs_emergency/README.md) for detailed setup instructions, IAM configuration, dynamic label extraction, and IRSA setup on EKS.

## CI/CD

This project includes automated end-to-end tests that run on every push to any branch (excluding tags):

- **GitHub Actions Workflow**: `.github/workflows/e2e-tests.yml`
- **Test Environment**: Kind (Kubernetes in Docker) cluster
- **Timeout**: 5 minutes maximum
- **Concurrency**: Previous runs are automatically cancelled when a new commit is pushed

The e2e tests validate that the metric-reader can be successfully deployed to a Kubernetes cluster alongside Prometheus and that the application starts correctly.

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
  -e THRESHOLD_OPERATOR="greater_than" \
  -e SOFT_THRESHOLD="80" \
  -e HARD_THRESHOLD="100" \
  -e SOFT_THRESHOLD_PLUGIN="log_action" \
  -e HARD_THRESHOLD_PLUGIN="file_action" \
  -e THRESHOLD_DURATION="5m" \
  -e PLUGIN_DIR="/plugins" \
  -v /path/to/plugins:/plugins \
  metric-reader
```

## Example Use Cases

### AWS EFS Burst Credit Monitoring

Monitor AWS EFS burst credits and automatically generate I/O activity to increase credits when they fall below a threshold.

## Creating Custom Plugins

See the [plugins README](plugins/README.md) for information on creating custom plugins.
