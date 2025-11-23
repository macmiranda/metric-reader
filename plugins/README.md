# Metric Reader Plugins

This directory contains plugins for the metric-reader service that can be executed when metric thresholds are exceeded. Plugins are loaded dynamically at runtime and can perform custom actions based on metric values.

## Plugin Interface

All plugins must implement the `ActionPlugin` interface:

```go
type ActionPlugin interface {
    // Execute is called when a threshold is crossed and maintained for the specified duration
    Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error
    // Name returns the name of the plugin
    Name() string
    // ValidateConfig validates that all required configuration for the plugin is present
    // Returns an error if configuration is invalid or missing required values
    ValidateConfig() error
}
```

**Note:** The `ValidateConfig()` method is called immediately after the plugin is loaded. It should validate that all required configuration is present and return an error if anything is missing or invalid. This allows the application to fail fast at startup with clear error messages, rather than at runtime when the plugin is executed.

## Creating a Plugin

1. Create a new directory for your plugin:
   ```bash
   mkdir plugins/my_plugin
   ```

2. Create a Go file with the following structure:
   ```go
   package main

   import (
       "context"
       "fmt"
       "os"
       "time"
   )

   type MyPlugin struct {
       // Add your plugin configuration here
       requiredConfig string
   }

   func (p *MyPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
       // Implement your plugin logic here
       return nil
   }

   func (p *MyPlugin) Name() string {
       return "my_plugin"
   }

   func (p *MyPlugin) ValidateConfig() error {
       // Validate that required configuration is present
       if p.requiredConfig == "" {
           return fmt.Errorf("MY_PLUGIN_CONFIG is required but not set")
       }
       return nil
   }

   // Plugin is the exported plugin symbol
   var Plugin MyPlugin

   func init() {
       // Initialize your plugin here
       requiredConfig := os.Getenv("MY_PLUGIN_CONFIG")
       Plugin = MyPlugin{
           requiredConfig: requiredConfig,
       }
   }
   ```

3. Build the plugin:
   ```bash
   go build -buildmode=plugin -o my_plugin.so my_plugin.go
   ```

## Available Plugins

### File Action Plugin

Creates a file of configurable size when a metric threshold is exceeded.

**Configuration:**

- `FILE_ACTION_DIR`: Directory where files will be created (default: `/tmp/metric-files`)
- `FILE_ACTION_SIZE`: Size of files to create in bytes (default: 1MB)

### Log Action Plugin

Logs threshold events with detailed information about the metric value and duration.

### EFS Emergency Plugin

Switches an AWS EFS filesystem from bursting throughput mode to elastic throughput mode when metric thresholds are exceeded. Designed for emergency situations where burst credits are depleted.

**Configuration (via config file or environment variables):**

- `efs_file_system_id` / `EFS_FILE_SYSTEM_ID`: The EFS filesystem ID (static - optional if using label)
- `efs_file_system_prometheus_label` / `EFS_FILE_SYSTEM_PROMETHEUS_LABEL`: Prometheus metric label name to extract filesystem ID from (optional if using static ID)
- `aws_region` / `AWS_REGION`: AWS region where the filesystem is located (optional, auto-detected)
- `prometheus_endpoint` / `PROMETHEUS_ENDPOINT`: Prometheus server URL (optional, default: `http://prometheus:9090`)

**Requirements:**

- AWS credentials (supports IRSA on EKS)
- IAM permissions: `elasticfilesystem:UpdateFileSystem`, `elasticfilesystem:DescribeFileSystems`

**Features:**
- Supports dynamic filesystem ID extraction from Prometheus metric labels
- Fallback to static configuration if label query fails
- Configurable via config file or environment variables

See [efs_emergency/README.md](efs_emergency/README.md) for detailed documentation.

## Using Plugins

1. Build your plugin as a shared library (`.so` file)
2. Place the `.so` file in the plugin directory
3. Set the `PLUGIN_DIR` environment variable to point to the directory containing your plugins
4. Specify which plugin to use with `SOFT_THRESHOLD_PLUGIN` and/or `HARD_THRESHOLD_PLUGIN`

**Important:** Only plugins that are explicitly specified in `SOFT_THRESHOLD_PLUGIN` or `HARD_THRESHOLD_PLUGIN` will be loaded. This improves performance and security by not loading unnecessary plugins.

Example with soft threshold:

```bash
PLUGIN_DIR=/path/to/plugins \
SOFT_THRESHOLD_PLUGIN=my_plugin \
SOFT_THRESHOLD=50 \
./metric-reader
```

Example with both soft and hard thresholds:

```bash
PLUGIN_DIR=/path/to/plugins \
SOFT_THRESHOLD_PLUGIN=log_action \
SOFT_THRESHOLD=50 \
HARD_THRESHOLD_PLUGIN=my_plugin \
HARD_THRESHOLD=90 \
./metric-reader
```

## Plugin Development Tips

1. **Validation**: Always implement `ValidateConfig()` to check required configuration at startup
2. **Error Handling**: Always return meaningful errors from your `Execute` method
3. **Context Usage**: Use the provided context for cancellation and timeouts
4. **Configuration**: Use environment variables for plugin configuration
5. **Logging**: Use the zerolog package for consistent logging
6. **Testing**: Test your plugin thoroughly before deployment
7. **Fail Fast**: Use `ValidateConfig()` to detect configuration issues early, before the plugin is executed

## Example Plugin

Here's a complete example of a simple plugin that sends HTTP requests:

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "time"
)

type HTTPPlugin struct {
    endpoint string
}

func (p *HTTPPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
    req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, nil)
    if err != nil {
        return fmt.Errorf("failed to create request: %v", err)
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %v", err)
    }
    defer resp.Body.Close()

    return nil
}

func (p *HTTPPlugin) Name() string {
    return "http_plugin"
}

func (p *HTTPPlugin) ValidateConfig() error {
    if p.endpoint == "" {
        return fmt.Errorf("HTTP_PLUGIN_ENDPOINT is required but not set")
    }
    return nil
}

var Plugin HTTPPlugin

func init() {
    endpoint := os.Getenv("HTTP_PLUGIN_ENDPOINT")
    if endpoint == "" {
        endpoint = "http://localhost:8080"
    }
    Plugin = HTTPPlugin{endpoint: endpoint}
}
```
