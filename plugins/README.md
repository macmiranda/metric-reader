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
}
```

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
       "time"
   )

   type MyPlugin struct {
       // Add your plugin configuration here
   }

   func (p *MyPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
       // Implement your plugin logic here
       return nil
   }

   func (p *MyPlugin) Name() string {
       return "my_plugin"
   }

   // Plugin is the exported plugin symbol
   var Plugin MyPlugin

   func init() {
       // Initialize your plugin here
       Plugin = MyPlugin{}
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

**Configuration:**

- `EFS_FILE_SYSTEM_ID`: The EFS filesystem ID (required)
- `AWS_REGION`: AWS region where the filesystem is located (optional, auto-detected)

**Requirements:**

- AWS credentials (supports IRSA on EKS)
- IAM permissions: `elasticfilesystem:UpdateFileSystem`, `elasticfilesystem:DescribeFileSystems`

See [efs_emergency/README.md](efs_emergency/README.md) for detailed documentation.

## Using Plugins

1. Build your plugin as a shared library (`.so` file)
2. Place the `.so` file in the plugin directory
3. Set the `PLUGIN_DIR` environment variable to point to the directory containing your plugins
4. Set the `ACTION_PLUGIN` environment variable to the name of your plugin

Example:

```bash
PLUGIN_DIR=/path/to/plugins ACTION_PLUGIN=my_plugin ./metric-reader
```

## Plugin Development Tips

1. **Error Handling**: Always return meaningful errors from your `Execute` method
2. **Context Usage**: Use the provided context for cancellation and timeouts
3. **Configuration**: Use environment variables for plugin configuration
4. **Logging**: Use the zerolog package for consistent logging
5. **Testing**: Test your plugin thoroughly before deployment

## Example Plugin

Here's a complete example of a simple plugin that sends HTTP requests:

```go
package main

import (
    "context"
    "fmt"
    "net/http"
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

var Plugin HTTPPlugin

func init() {
    endpoint := os.Getenv("HTTP_PLUGIN_ENDPOINT")
    if endpoint == "" {
        endpoint = "http://localhost:8080"
    }
    Plugin = HTTPPlugin{endpoint: endpoint}
}
```
