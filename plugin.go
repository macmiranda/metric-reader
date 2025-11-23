package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	// ValidateConfig validates that all required configuration for the plugin is present
	// Returns an error if configuration is invalid or missing required values
	ValidateConfig() error
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

		pluginPath := filepath.Join(dir, entry.Name())
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

// LoadRequiredPlugins loads only the specified plugins from a directory and validates their configuration
func LoadRequiredPlugins(dir string, requiredPlugins map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %v", err)
	}

	loadedPlugins := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}

		// Extract plugin name from filename (remove .so extension)
		pluginName := strings.TrimSuffix(entry.Name(), ".so")
		
		// Only load the plugin if it's required
		if !requiredPlugins[pluginName] {
			log.Debug().Str("plugin", pluginName).Msg("plugin skipped - not required")
			continue
		}

		pluginPath := filepath.Join(dir, entry.Name())
		plugin, err := LoadPlugin(pluginPath)
		if err != nil {
			log.Error().Err(err).Str("plugin", entry.Name()).Msg("failed to load plugin")
			continue
		}

		// Verify the plugin name matches the expected name from filename
		if plugin.Name() != pluginName {
			log.Warn().
				Str("expected", pluginName).
				Str("actual", plugin.Name()).
				Msg("plugin name mismatch - plugin filename should match plugin Name() method")
		}

		// Validate plugin configuration before registering
		if err := plugin.ValidateConfig(); err != nil {
			return fmt.Errorf("plugin '%s' configuration validation failed: %v", plugin.Name(), err)
		}

		RegisterPlugin(plugin)
		loadedPlugins[plugin.Name()] = true
		log.Info().Str("plugin", plugin.Name()).Msg("plugin loaded and validated successfully")
	}

	// Check that all required plugins were found and loaded
	for pluginName := range requiredPlugins {
		if !loadedPlugins[pluginName] {
			return fmt.Errorf("required plugin '%s' not found in directory '%s'", pluginName, dir)
		}
	}

	return nil
}
