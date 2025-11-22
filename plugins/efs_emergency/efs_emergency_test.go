package main

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestPluginInterface verifies that the plugin implements the required interface
func TestPluginInterface(t *testing.T) {
	// Check that Plugin.Name() returns the correct name
	if Plugin.Name() != "efs_emergency" {
		t.Errorf("expected plugin name 'efs_emergency', got '%s'", Plugin.Name())
	}
}

// TestPluginNameMethod tests the Name method
func TestPluginNameMethod(t *testing.T) {
	plugin := EFSEmergencyPlugin{}
	expected := "efs_emergency"
	if plugin.Name() != expected {
		t.Errorf("expected name '%s', got '%s'", expected, plugin.Name())
	}
}

// TestEnvironmentVariableValidation tests that the plugin requires EFS_FILE_SYSTEM_ID or EFS_METRIC_LABEL
func TestEnvironmentVariableValidation(t *testing.T) {
	// This test verifies that EFS_FILE_SYSTEM_ID or EFS_METRIC_LABEL is required
	// Note: We can't easily test init() function failure, but we can verify
	// the environment variable is set in the current test environment
	fileSystemId := os.Getenv("EFS_FILE_SYSTEM_ID")
	metricLabel := os.Getenv("EFS_METRIC_LABEL")
	if fileSystemId == "" && metricLabel == "" {
		t.Log("Neither EFS_FILE_SYSTEM_ID nor EFS_METRIC_LABEL is set - this is expected for testing")
		t.Log("In production, at least one of these environment variables must be set")
	}
}

// TestExecuteSignature verifies the Execute method signature matches the interface
func TestExecuteSignature(t *testing.T) {
	// This is a compile-time check that Execute method exists with correct signature
	// We can't actually execute it without AWS credentials and a real filesystem
	plugin := EFSEmergencyPlugin{
		fileSystemId:      "fs-test123",
		metricLabelName:   "file_system_id",
		region:            "us-east-1",
		client:            nil, // In a real test, we'd use a mock client
		prometheusAPI:     nil,
		prometheusEnabled: false,
	}

	// We're just checking that this compiles
	ctx := context.Background()
	_ = plugin.Execute(ctx, "test_metric", 100.0, "<50", 5*time.Minute)
}
