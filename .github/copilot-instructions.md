# Copilot Instructions for metric-reader

**Project:** Lightweight Go application that monitors Prometheus metrics and executes configurable actions when thresholds are exceeded. Runs as a Kubernetes sidecar or standalone service.

**Status:** Alpha (pre-1.0) - breaking changes acceptable, focus on simplicity over backward compatibility.

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
    Str("metric_name", metricName).
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

## How to Contribute

**Workflow:**
1. Create feature branch
2. Make focused, incremental changes
3. Test: `just run-tests` or `go test -v ./...`
4. Build to verify compilation
5. Run `go fmt` and `go vet`
6. Clear commit messages (reference issue numbers)

**Testing:**
- Unit tests exist (see `*_test.go` files)
- Manual testing: Docker Compose or Kind cluster
- Verify plugin loading and leader election

**Documentation (Required):**
- **README.md** - user-facing features
- **.github/copilot-instructions.md** - implementation details
- **deployment files** - `docker-compose.yml`, `kubernetes/metric-reader.yaml` when config changes
- **config.toml.example** - new configuration sections
- **Plugin README.md** - each plugin needs its own README

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
- **Required:** `METRIC_NAME`
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
- **Leader election fails:** Check RBAC, verify `POD_NAMESPACE` set via Downward API
- **Metrics not read:** Verify Prometheus endpoint, check metric name/labels, use `LOG_LEVEL=debug`

## Additional Resources

- [Plugin Development Guide](../plugins/README.md)
- [Prometheus Query Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Zerolog Documentation](https://github.com/rs/zerolog)
