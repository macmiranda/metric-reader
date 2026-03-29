# Agent Instructions for metric-reader

**Project:** Lightweight Go application that monitors Prometheus metrics and executes configurable actions when thresholds are exceeded. Runs as a Kubernetes sidecar or standalone service.

**Status:** Alpha (pre-1.0) - breaking changes acceptable, focus on simplicity over backward compatibility.

## Breaking Changes

### Threshold configuration requires `[soft]` and `[hard]` sections

The configuration now requires `[soft]` and `[hard]` sections for threshold configuration. Each section has its own `threshold`, `plugin`, `duration`, and `backoff_delay` settings.

**Before:**
```toml
soft_threshold = 80.0
soft_plugin = "log_action"
hard_threshold = 100.0
hard_plugin = "file_action"
```

**After:**
```toml
[soft]
threshold = 80.0
plugin = "log_action"
duration = "30s"
backoff_delay = "1m"

[hard]
threshold = 100.0
plugin = "file_action"
duration = "30s"
backoff_delay = "1m"
```

### PROMETHEUS_QUERY replaces METRIC_NAME + LABEL_FILTERS

The `METRIC_NAME` and `LABEL_FILTERS` environment variables (and their corresponding `metric_name` / `label_filters` config file keys) have been removed and replaced with a single `PROMETHEUS_QUERY` field.

**Before:**
```toml
metric_name = "up"
label_filters = 'job="prometheus",instance="localhost:9090"'
```

**After:**
```toml
prometheus_query = 'up{job="prometheus",instance="localhost:9090"}'
```

This change supports full PromQL expressions, including:
- Simple metric names: `up`
- Metric names with label filters: `up{job="prometheus"}`
- Functions and operators: `rate(http_requests_total[5m])`
- Aggregations: `avg(node_memory_MemAvailable_bytes) / avg(node_memory_MemTotal_bytes) * 100`

The query result must be a scalar or single-element vector for threshold comparison to work correctly.

## Features

- Monitor any Prometheus metric with soft/hard thresholds
- Plugin system for custom actions (`.so` files with `ActionPlugin` interface)
- State machine for threshold transitions (NotBreached → SoftThresholdActive → HardThresholdActive)
- Leader election for multiple replicas (Kubernetes coordination leases)
- Built-in plugins: `log_action`, `file_action`, `efs_emergency`
- Configuration via TOML files or environment variables
- Selective plugin loading - only specified plugins are loaded

## Key Dependencies

- `github.com/prometheus/client_golang` - Prometheus client
- `github.com/rs/zerolog` - Structured logging
- `github.com/spf13/viper` - Configuration management
- `github.com/aws/aws-sdk-go-v2` - AWS SDK (for EFS plugin)
- `k8s.io/client-go` - Kubernetes client for leader election
- Go 1.23+ (as specified in go.mod)

## Build & Test

```bash
# Using Just (recommended)
just build              # Build main application
just build-plugins      # Build plugin .so files
just run-tests         # Run all tests

# Direct Go commands
go build -o metric-reader .
go build -buildmode=plugin -o plugins/<name>.so plugins/<name>/<name>.go
go test -v ./...
```

## End-to-End Tests

The e2e test harness is defined in `.github/workflows/e2e-tests.yml` and can be run locally using the Just recipes below. It spins up a Kind (Kubernetes-in-Docker) cluster, loads the built image, deploys all resources, and validates the deployment.

**Prerequisites:** Docker, Kind, kubectl, and Just must be installed.

```bash
# Full e2e test (mirrors the CI workflow)
just e2e-test

# Step-by-step (useful for debugging)
just build-image          # Build Docker image
just kind-up              # Create Kind cluster + load image + install metrics-server
just k8s-apply            # Deploy metric-reader + Prometheus to the cluster
just k8s-wait             # Wait for all pods to become Ready (120s timeout)
just k8s-status           # Check pod status
just k8s-logs             # Tail metric-reader logs
just kind-down            # Tear down the Kind cluster
```

**CI workflow steps (`.github/workflows/e2e-tests.yml`):**
1. Build application: `just build`
2. Build plugins: `just build-plugins`
3. Run e2e tests: `just e2e-test` (builds image, creates Kind cluster, deploys, waits, validates)
4. On failure: prints metric-reader and Prometheus logs and deployment status
5. Always: tears down the Kind cluster with `just kind-down`

## Coding Style

**General:**
- Follow standard Go conventions and idioms
- Use `zerolog` for all logging (imported as `log`)
- Handle errors explicitly with context: `fmt.Errorf("description: %v", err)`
- Use context for cancellation and timeouts
- Descriptive variable names over abbreviations
- Run `go fmt` before committing

**Logging:**
```go
log.Info().
    Str("query", query).
    Float64("value", value).
    Msg("processing metric")
```
Levels: `Debug` (diagnostics), `Info` (operational), `Warn` (review needed), `Error` (prevents operation), `Fatal` (program termination)

**Error Handling:**
- Return errors with context
- Log before returning when appropriate
- Use `log.Fatal()` only for startup configuration errors

## Plugin Development

**Interface:**
```go
type ActionPlugin interface {
    Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error
    Name() string
}
```

**Requirements:**
1. Package must be `main`
2. Export variable `Plugin` of your plugin type
3. Build with `-buildmode=plugin`

**Configuration:**
- Preferred: TOML `[plugins.<name>]` sections
- Backward compatible: Environment variables with plugin name prefix
- Example:
```toml
[plugins.file_action]
dir = "/tmp/metric-files"
size = 1048576
```

**Best Practices:**
- Use provided context for cancellation
- Return meaningful errors
- Use `zerolog` for logging
- Provide sensible defaults
- Handle missing configuration gracefully

## Testing

**Test Structure:**
- Unit tests: `*_test.go` files alongside source code
- State machine tests: `state_machine_test.go` (comprehensive coverage)
- Plugin tests: `plugin_test.go`, per-plugin test files
- Config tests: `config_test.go` (TOML and env var validation)

**Running Tests:**
```bash
just run-tests           # Run all tests
go test -v ./...         # Direct go test command
go test -v -run TestName # Run specific test
```

**Test Coverage:**
- State machine transitions have extensive coverage
- Plugin loading and validation tests exist
- Configuration parsing tests for TOML and env vars
- Manual testing recommended for:
  - Docker Compose deployments
  - Kubernetes with leader election
  - Plugin execution in real environments

**Writing Tests:**
- Follow existing test patterns in `*_test.go` files
- Use table-driven tests for multiple scenarios
- Mock external dependencies (Prometheus, Kubernetes)
- Test both success and error paths
- Verify log output when appropriate

## Documentation

**Required Documentation Updates:**

When making changes, update relevant documentation:

1. **README.md** - User-facing documentation
   - Feature descriptions and usage examples
   - Configuration options and examples
   - Quick start guides
   - Deployment instructions

2. **AGENTS.md** - Developer documentation
   - Implementation details and architecture
   - Coding conventions and patterns
   - Development workflows
   - Internal APIs and structures

3. **config.toml.example** - Configuration template
   - Add new configuration sections
   - Include comments explaining options
   - Provide sensible example values

4. **Deployment files** - When config structure changes
   - `docker-compose.yml` - environment variables
   - `kubernetes/metric-reader.yaml` - ConfigMap structure

5. **Plugin README.md** - Each plugin needs its own
   - Plugin purpose and use cases
   - Configuration options (TOML and env vars)
   - IAM permissions (if applicable)
   - Usage examples

**Documentation Style:**
- Clear, concise language
- Code examples for complex concepts
- Both TOML and environment variable formats
- Include defaults and required vs optional

## How to Contribute

**Workflow:**
1. Create feature branch
2. Make focused, incremental changes
3. Test: see **Testing** section for details
4. Build to verify compilation
5. Run `go fmt` and `go vet`
6. Update documentation: see **Documentation** section
7. Clear commit messages (reference issue numbers)

**Adding a Plugin:**
1. Create `plugins/my_plugin/` directory
2. Implement `ActionPlugin` interface
3. Add to `Justfile` build commands
4. Add config struct in `config.go` (inside `PluginConfig` struct):
```go
// In PluginConfig struct
MyPlugin struct {
    Setting1 string `mapstructure:"setting1"`
    Setting2 int    `mapstructure:"setting2"`
} `mapstructure:"my_plugin"`
```
5. Bind environment variables in `LoadConfig()`:
```go
v.BindEnv("plugins.my_plugin.setting1", "MY_PLUGIN_SETTING1")
v.BindEnv("plugins.my_plugin.setting2", "MY_PLUGIN_SETTING2")
```
6. Document in plugin README and main README
7. Update `config.toml.example`

## Configuration

**Threshold Structure (Breaking Change in v0.x):**
```toml
threshold_operator = "greater_than"

[soft]
threshold = 80.0
plugin = "log_action"
duration = "30s"
backoff_delay = "1m"

[hard]
threshold = 100.0
plugin = "file_action"
duration = "30s"
backoff_delay = "1m"
```

**Environment Variables:**
- **Required:** `PROMETHEUS_QUERY`
- **Optional:** `PROMETHEUS_ENDPOINT` (default: `http://prometheus:9090`), `LOG_LEVEL` (default: `info`)
- **Thresholds:** `SOFT_THRESHOLD`, `SOFT_PLUGIN`, `SOFT_DURATION`, `SOFT_BACKOFF_DELAY`, `HARD_THRESHOLD`, `HARD_PLUGIN`, `HARD_DURATION`, `HARD_BACKOFF_DELAY`
- **Leader election:** `LEADER_ELECTION_ENABLED` (default: `true`), `LEADER_ELECTION_LOCK_NAME`
- **Missing values:** `MISSING_VALUE_BEHAVIOR` (`last_value`, `zero`, `assume_breached`)

## Improvements & Future Work

- Expand test coverage beyond state machine tests
- Add more built-in plugins
- Configuration validation improvements
- Metrics exposition for monitoring the monitor
- Support for multiple metrics in a single instance
- Plugin hot-reloading without restart

## Troubleshooting

- **Plugin not loading:** Verify Go version match, check `PLUGIN_DIR`, review logs
- **Leader election fails:** Check RBAC
- **Metrics not read:** Verify Prometheus endpoint, check query and labels, use `LOG_LEVEL=debug`

## Additional Resources

- [Plugin Development Guide](plugins/README.md)
- [Prometheus Query Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Zerolog Documentation](https://github.com/rs/zerolog)

