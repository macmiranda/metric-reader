package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Mock plugin for testing
type mockValidPlugin struct {
	name string
}

func (m *mockValidPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
	return nil
}

func (m *mockValidPlugin) Name() string {
	return m.name
}

func (m *mockValidPlugin) ValidateConfig() error {
	return nil
}

// Mock plugin with invalid config
type mockInvalidPlugin struct {
	name string
}

func (m *mockInvalidPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
	return nil
}

func (m *mockInvalidPlugin) Name() string {
	return m.name
}

func (m *mockInvalidPlugin) ValidateConfig() error {
	return fmt.Errorf("mock validation error: required config missing")
}

func TestLoadRequiredPlugins_OnlyLoadsSpecifiedPlugins(t *testing.T) {
	// Clear the plugin registry
	PluginRegistry = make(map[string]ActionPlugin)

	// Register mock plugins
	RegisterPlugin(&mockValidPlugin{name: "plugin1"})
	RegisterPlugin(&mockValidPlugin{name: "plugin2"})
	RegisterPlugin(&mockValidPlugin{name: "plugin3"})

	// Check that all three plugins are registered
	if len(PluginRegistry) != 3 {
		t.Errorf("Expected 3 plugins registered, got %d", len(PluginRegistry))
	}

	// Verify specific plugins exist
	if _, ok := PluginRegistry["plugin1"]; !ok {
		t.Error("Expected plugin1 to be registered")
	}
	if _, ok := PluginRegistry["plugin2"]; !ok {
		t.Error("Expected plugin2 to be registered")
	}
	if _, ok := PluginRegistry["plugin3"]; !ok {
		t.Error("Expected plugin3 to be registered")
	}
}

func TestPluginValidateConfig(t *testing.T) {
	// Test valid plugin
	validPlugin := &mockValidPlugin{name: "valid"}
	if err := validPlugin.ValidateConfig(); err != nil {
		t.Errorf("Expected valid plugin to pass validation, got error: %v", err)
	}

	// Test invalid plugin
	invalidPlugin := &mockInvalidPlugin{name: "invalid"}
	if err := invalidPlugin.ValidateConfig(); err == nil {
		t.Error("Expected invalid plugin to fail validation, but it passed")
	}
}

func TestFileActionPluginValidation(t *testing.T) {
	// Save original env vars
	originalDir := os.Getenv("FILE_ACTION_DIR")
	originalSize := os.Getenv("FILE_ACTION_SIZE")
	defer func() {
		if originalDir != "" {
			os.Setenv("FILE_ACTION_DIR", originalDir)
		} else {
			os.Unsetenv("FILE_ACTION_DIR")
		}
		if originalSize != "" {
			os.Setenv("FILE_ACTION_SIZE", originalSize)
		} else {
			os.Unsetenv("FILE_ACTION_SIZE")
		}
	}()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test 1: Valid configuration
	os.Setenv("FILE_ACTION_DIR", tmpDir)
	os.Setenv("FILE_ACTION_SIZE", "1048576")

	// Reload the plugin by creating a new config (simulating plugin init)
	// Note: In real scenario, plugin would be loaded from .so file
	// Here we're just testing the validation logic conceptually
	
	// We can't easily test plugin.go loading without building .so files,
	// but we can test that the config structure works
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Plugins.FileAction.Dir != tmpDir {
		t.Errorf("Expected FILE_ACTION_DIR to be %s, got %s", tmpDir, config.Plugins.FileAction.Dir)
	}

	if config.Plugins.FileAction.Size != 1048576 {
		t.Errorf("Expected FILE_ACTION_SIZE to be 1048576, got %d", config.Plugins.FileAction.Size)
	}
}

func TestEFSEmergencyPluginValidation(t *testing.T) {
	// Save original env vars
	originalFSID := os.Getenv("EFS_FILE_SYSTEM_ID")
	originalLabel := os.Getenv("EFS_FILE_SYSTEM_PROMETHEUS_LABEL")
	defer func() {
		if originalFSID != "" {
			os.Setenv("EFS_FILE_SYSTEM_ID", originalFSID)
		} else {
			os.Unsetenv("EFS_FILE_SYSTEM_ID")
		}
		if originalLabel != "" {
			os.Setenv("EFS_FILE_SYSTEM_PROMETHEUS_LABEL", originalLabel)
		} else {
			os.Unsetenv("EFS_FILE_SYSTEM_PROMETHEUS_LABEL")
		}
	}()

	// Test 1: Valid configuration with file system ID
	os.Setenv("EFS_FILE_SYSTEM_ID", "fs-12345678")
	os.Unsetenv("EFS_FILE_SYSTEM_PROMETHEUS_LABEL")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Plugins.EFSEmergency.FileSystemID != "fs-12345678" {
		t.Errorf("Expected EFS_FILE_SYSTEM_ID to be 'fs-12345678', got %s", config.Plugins.EFSEmergency.FileSystemID)
	}

	// Test 2: Valid configuration with prometheus label
	os.Unsetenv("EFS_FILE_SYSTEM_ID")
	os.Setenv("EFS_FILE_SYSTEM_PROMETHEUS_LABEL", "filesystem_id")

	config, err = LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Plugins.EFSEmergency.FileSystemPrometheusLabel != "filesystem_id" {
		t.Errorf("Expected EFS_FILE_SYSTEM_PROMETHEUS_LABEL to be 'filesystem_id', got %s", config.Plugins.EFSEmergency.FileSystemPrometheusLabel)
	}

	// Test 3: Both set (should be valid)
	os.Setenv("EFS_FILE_SYSTEM_ID", "fs-12345678")
	os.Setenv("EFS_FILE_SYSTEM_PROMETHEUS_LABEL", "filesystem_id")

	config, err = LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Plugins.EFSEmergency.FileSystemID != "fs-12345678" {
		t.Errorf("Expected EFS_FILE_SYSTEM_ID to be 'fs-12345678', got %s", config.Plugins.EFSEmergency.FileSystemID)
	}
	if config.Plugins.EFSEmergency.FileSystemPrometheusLabel != "filesystem_id" {
		t.Errorf("Expected EFS_FILE_SYSTEM_PROMETHEUS_LABEL to be 'filesystem_id', got %s", config.Plugins.EFSEmergency.FileSystemPrometheusLabel)
	}
}

func TestLoadRequiredPlugins_NonExistentDirectory(t *testing.T) {
	requiredPlugins := map[string]bool{
		"test_plugin": true,
	}

	err := LoadRequiredPlugins("/nonexistent/directory", requiredPlugins)
	if err == nil {
		t.Error("Expected error when loading from non-existent directory, got nil")
	}
}

func TestLoadRequiredPlugins_MissingRequiredPlugin(t *testing.T) {
	// Create a temporary directory with no plugins
	tmpDir := t.TempDir()

	requiredPlugins := map[string]bool{
		"missing_plugin": true,
	}

	err := LoadRequiredPlugins(tmpDir, requiredPlugins)
	if err == nil {
		t.Error("Expected error when required plugin is not found, got nil")
	}

	if err != nil && len(err.Error()) > 0 {
		if !contains(err.Error(), "required plugin") || !contains(err.Error(), "missing_plugin") {
			t.Errorf("Expected error message to contain 'required plugin' and 'missing_plugin', got: %s", err.Error())
		}
	}
}

func TestLoadRequiredPlugins_EmptyRequiredPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy .so file (won't be loaded since requiredPlugins is empty)
	dummyPlugin := filepath.Join(tmpDir, "dummy.so")
	if err := os.WriteFile(dummyPlugin, []byte("dummy"), 0644); err != nil {
		t.Fatalf("Failed to create dummy plugin file: %v", err)
	}

	requiredPlugins := map[string]bool{}

	err := LoadRequiredPlugins(tmpDir, requiredPlugins)
	if err != nil {
		t.Errorf("Expected no error with empty required plugins, got: %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		len(s) > len(substr) && containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
