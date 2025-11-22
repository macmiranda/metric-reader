package main

import (
	"os"
	"testing"
)

func TestLockNamespaceConfig(t *testing.T) {
	// Save original env vars
	originalLockNamespace := os.Getenv("LEADER_ELECTION_LOCK_NAMESPACE")
	defer func() {
		if originalLockNamespace != "" {
			os.Setenv("LEADER_ELECTION_LOCK_NAMESPACE", originalLockNamespace)
		} else {
			os.Unsetenv("LEADER_ELECTION_LOCK_NAMESPACE")
		}
	}()

	// Test 1: Default config (no env vars)
	os.Unsetenv("LEADER_ELECTION_LOCK_NAMESPACE")
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Test 1 failed: %v", err)
	}
	if config.LeaderElectionLockNamespace != "" {
		t.Errorf("Test 1 failed: expected empty LeaderElectionLockNamespace, got %q", config.LeaderElectionLockNamespace)
	}

	// Test 2: Set via environment variable
	os.Setenv("LEADER_ELECTION_LOCK_NAMESPACE", "test-namespace")
	config, err = LoadConfig()
	if err != nil {
		t.Fatalf("Test 2 failed: %v", err)
	}
	if config.LeaderElectionLockNamespace != "test-namespace" {
		t.Errorf("Test 2 failed: expected 'test-namespace', got %q", config.LeaderElectionLockNamespace)
	}
}

func TestPluginConfigStructure(t *testing.T) {
	// Save original env vars
	originalFileActionDir := os.Getenv("FILE_ACTION_DIR")
	originalFileActionSize := os.Getenv("FILE_ACTION_SIZE")
	originalEFSFileSystemID := os.Getenv("EFS_FILE_SYSTEM_ID")
	originalAWSRegion := os.Getenv("AWS_REGION")

	defer func() {
		// Restore original env vars
		if originalFileActionDir != "" {
			os.Setenv("FILE_ACTION_DIR", originalFileActionDir)
		} else {
			os.Unsetenv("FILE_ACTION_DIR")
		}
		if originalFileActionSize != "" {
			os.Setenv("FILE_ACTION_SIZE", originalFileActionSize)
		} else {
			os.Unsetenv("FILE_ACTION_SIZE")
		}
		if originalEFSFileSystemID != "" {
			os.Setenv("EFS_FILE_SYSTEM_ID", originalEFSFileSystemID)
		} else {
			os.Unsetenv("EFS_FILE_SYSTEM_ID")
		}
		if originalAWSRegion != "" {
			os.Setenv("AWS_REGION", originalAWSRegion)
		} else {
			os.Unsetenv("AWS_REGION")
		}
	}()

	// Test 1: Default plugin configuration
	os.Unsetenv("FILE_ACTION_DIR")
	os.Unsetenv("FILE_ACTION_SIZE")
	os.Unsetenv("EFS_FILE_SYSTEM_ID")
	os.Unsetenv("AWS_REGION")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Test 1 failed: %v", err)
	}

	// Check nested structure defaults
	if config.Plugins.FileAction.Dir != "/tmp/metric-files" {
		t.Errorf("Test 1 failed: expected default plugins.file_action.dir '/tmp/metric-files', got %q", config.Plugins.FileAction.Dir)
	}
	if config.Plugins.FileAction.Size != 1024*1024 {
		t.Errorf("Test 1 failed: expected default plugins.file_action.size 1048576, got %d", config.Plugins.FileAction.Size)
	}

	// Check backward compatibility - old fields should also be populated
	if config.FileActionDir != "/tmp/metric-files" {
		t.Errorf("Test 1 failed: expected backward compatible file_action_dir '/tmp/metric-files', got %q", config.FileActionDir)
	}
	if config.FileActionSize != 1024*1024 {
		t.Errorf("Test 1 failed: expected backward compatible file_action_size 1048576, got %d", config.FileActionSize)
	}

	// Test 2: Set plugin config via environment variables
	os.Setenv("FILE_ACTION_DIR", "/custom/path")
	os.Setenv("FILE_ACTION_SIZE", "2097152") // 2MB
	os.Setenv("EFS_FILE_SYSTEM_ID", "fs-12345")
	os.Setenv("AWS_REGION", "us-west-2")

	config, err = LoadConfig()
	if err != nil {
		t.Fatalf("Test 2 failed: %v", err)
	}

	// Check nested structure gets values from env vars
	if config.Plugins.FileAction.Dir != "/custom/path" {
		t.Errorf("Test 2 failed: expected plugins.file_action.dir '/custom/path', got %q", config.Plugins.FileAction.Dir)
	}
	if config.Plugins.FileAction.Size != 2097152 {
		t.Errorf("Test 2 failed: expected plugins.file_action.size 2097152, got %d", config.Plugins.FileAction.Size)
	}
	if config.Plugins.EFSEmergency.FileSystemID != "fs-12345" {
		t.Errorf("Test 2 failed: expected plugins.efs_emergency.file_system_id 'fs-12345', got %q", config.Plugins.EFSEmergency.FileSystemID)
	}
	if config.Plugins.EFSEmergency.AWSRegion != "us-west-2" {
		t.Errorf("Test 2 failed: expected plugins.efs_emergency.aws_region 'us-west-2', got %q", config.Plugins.EFSEmergency.AWSRegion)
	}

	// Check backward compatibility - old fields should also get values
	if config.FileActionDir != "/custom/path" {
		t.Errorf("Test 2 failed: expected backward compatible file_action_dir '/custom/path', got %q", config.FileActionDir)
	}
	if config.FileActionSize != 2097152 {
		t.Errorf("Test 2 failed: expected backward compatible file_action_size 2097152, got %d", config.FileActionSize)
	}
	if config.EFSFileSystemID != "fs-12345" {
		t.Errorf("Test 2 failed: expected backward compatible efs_file_system_id 'fs-12345', got %q", config.EFSFileSystemID)
	}
	if config.AWSRegion != "us-west-2" {
		t.Errorf("Test 2 failed: expected backward compatible aws_region 'us-west-2', got %q", config.AWSRegion)
	}
}

func TestNestedTOMLConfig(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Clear any existing env vars
	envVars := []string{"FILE_ACTION_DIR", "FILE_ACTION_SIZE", "EFS_FILE_SYSTEM_ID", "AWS_REGION"}
	savedEnvs := make(map[string]string)
	for _, key := range envVars {
		savedEnvs[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		for key, value := range savedEnvs {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	
	// Create a config file with nested structure
	configContent := `log_level = "debug"
metric_name = "test_metric"
threshold_operator = "greater_than"

[plugins.file_action]
dir = "/test/nested/path"
size = 5242880

[plugins.efs_emergency]
file_system_id = "fs-nested-test"
aws_region = "eu-west-1"
`
	
	configPath := tmpDir + "/config.toml"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	
	// Change to temp directory so config file is found
	os.Chdir(tmpDir)
	
	// Load config
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Verify nested structure is loaded
	if config.Plugins.FileAction.Dir != "/test/nested/path" {
		t.Errorf("Expected nested plugins.file_action.dir '/test/nested/path', got %q", config.Plugins.FileAction.Dir)
	}
	if config.Plugins.FileAction.Size != 5242880 {
		t.Errorf("Expected nested plugins.file_action.size 5242880, got %d", config.Plugins.FileAction.Size)
	}
	if config.Plugins.EFSEmergency.FileSystemID != "fs-nested-test" {
		t.Errorf("Expected nested plugins.efs_emergency.file_system_id 'fs-nested-test', got %q", config.Plugins.EFSEmergency.FileSystemID)
	}
	if config.Plugins.EFSEmergency.AWSRegion != "eu-west-1" {
		t.Errorf("Expected nested plugins.efs_emergency.aws_region 'eu-west-1', got %q", config.Plugins.EFSEmergency.AWSRegion)
	}
	
	// Verify backward compatibility fields are also populated
	if config.FileActionDir != "/test/nested/path" {
		t.Errorf("Expected backward compatible file_action_dir '/test/nested/path', got %q", config.FileActionDir)
	}
	if config.FileActionSize != 5242880 {
		t.Errorf("Expected backward compatible file_action_size 5242880, got %d", config.FileActionSize)
	}
	if config.EFSFileSystemID != "fs-nested-test" {
		t.Errorf("Expected backward compatible efs_file_system_id 'fs-nested-test', got %q", config.EFSFileSystemID)
	}
	if config.AWSRegion != "eu-west-1" {
		t.Errorf("Expected backward compatible aws_region 'eu-west-1', got %q", config.AWSRegion)
	}
}
