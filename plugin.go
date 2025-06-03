package main

import (
	"context"
	"fmt"
	"os"
	"plugin"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ActionPlugin defines the interface that all threshold action plugins must implement
type ActionPlugin interface {
	// Execute is called when a threshold is crossed and maintained for the specified duration
	Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error
	// Name returns the name of the plugin
	Name() string
}

// PluginRegistry holds all registered plugins
var PluginRegistry = make(map[string]ActionPlugin)

// RegisterPlugin adds a plugin to the registry
func RegisterPlugin(p ActionPlugin) {
	PluginRegistry[p.Name()] = p
}

// LoadPlugin loads a plugin from a shared library file
func LoadPlugin(pluginPath string) (ActionPlugin, error) {
	p, err := plugin.Open(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin: %v", err)
	}

	symPlugin, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("plugin symbol not found: %v", err)
	}

	actionPlugin, ok := symPlugin.(ActionPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin does not implement ActionPlugin interface")
	}

	return actionPlugin, nil
}

// LoadPluginsFromDirectory loads all plugins from a directory
func LoadPluginsFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}

		pluginPath := fmt.Sprintf("%s/%s", dir, entry.Name())
		plugin, err := LoadPlugin(pluginPath)
		if err != nil {
			log.Error().Err(err).Str("plugin", entry.Name()).Msg("failed to load plugin")
			continue
		}

		RegisterPlugin(plugin)
		log.Info().Str("plugin", plugin.Name()).Msg("plugin loaded successfully")
	}

	return nil
}
