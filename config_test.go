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

func TestBackwardCompatibleFlatTOMLConfig(t *testing.T) {
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
	
	// Create a config file with old flat structure (backward compatibility)
	configContent := `log_level = "debug"
metric_name = "test_metric"
threshold_operator = "greater_than"

# Old flat structure for backward compatibility
file_action_dir = "/old/flat/path"
file_action_size = 3145728
efs_file_system_id = "fs-old-flat"
aws_region = "ap-south-1"
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
	
	// Verify old flat structure still works
	if config.FileActionDir != "/old/flat/path" {
		t.Errorf("Expected backward compatible file_action_dir '/old/flat/path', got %q", config.FileActionDir)
	}
	if config.FileActionSize != 3145728 {
		t.Errorf("Expected backward compatible file_action_size 3145728, got %d", config.FileActionSize)
	}
	if config.EFSFileSystemID != "fs-old-flat" {
		t.Errorf("Expected backward compatible efs_file_system_id 'fs-old-flat', got %q", config.EFSFileSystemID)
	}
	if config.AWSRegion != "ap-south-1" {
		t.Errorf("Expected backward compatible aws_region 'ap-south-1', got %q", config.AWSRegion)
	}
	
	// Verify new nested structure also gets populated
	if config.Plugins.FileAction.Dir != "/old/flat/path" {
		t.Errorf("Expected nested plugins.file_action.dir '/old/flat/path', got %q", config.Plugins.FileAction.Dir)
	}
	if config.Plugins.FileAction.Size != 3145728 {
		t.Errorf("Expected nested plugins.file_action.size 3145728, got %d", config.Plugins.FileAction.Size)
	}
	if config.Plugins.EFSEmergency.FileSystemID != "fs-old-flat" {
		t.Errorf("Expected nested plugins.efs_emergency.file_system_id 'fs-old-flat', got %q", config.Plugins.EFSEmergency.FileSystemID)
	}
	if config.Plugins.EFSEmergency.AWSRegion != "ap-south-1" {
		t.Errorf("Expected nested plugins.efs_emergency.aws_region 'ap-south-1', got %q", config.Plugins.EFSEmergency.AWSRegion)
	}
}

func TestNestedThresholdConfig(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Clear any existing threshold env vars
	thresholdEnvVars := []string{
		"SOFT_THRESHOLD", "HARD_THRESHOLD",
		"SOFT_THRESHOLD_PLUGIN", "HARD_THRESHOLD_PLUGIN",
		"SOFT_DURATION", "HARD_DURATION",
		"SOFT_BACKOFF_DELAY", "HARD_BACKOFF_DELAY",
		"THRESHOLD_DURATION", "BACKOFF_DELAY",
	}
	savedEnvs := make(map[string]string)
	for _, key := range thresholdEnvVars {
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
	
	// Create a config file with new nested threshold structure
	configContent := `log_level = "debug"
metric_name = "test_metric"
threshold_operator = "greater_than"

[soft]
threshold = 80.0
plugin = "log_action"
duration = "30s"
backoff_delay = "1m"

[hard]
threshold = 100.0
plugin = "file_action"
duration = "45s"
backoff_delay = "2m"
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
	
	// Verify soft threshold section
	if config.Soft == nil {
		t.Fatal("Expected Soft section to be set, got nil")
	}
	if config.Soft.Threshold != 80.0 {
		t.Errorf("Expected soft.threshold 80.0, got %f", config.Soft.Threshold)
	}
	if config.Soft.Plugin != "log_action" {
		t.Errorf("Expected soft.plugin 'log_action', got %q", config.Soft.Plugin)
	}
	if config.Soft.Duration.Seconds() != 30 {
		t.Errorf("Expected soft.duration 30s, got %v", config.Soft.Duration)
	}
	if config.Soft.BackoffDelay.Seconds() != 60 {
		t.Errorf("Expected soft.backoff_delay 1m, got %v", config.Soft.BackoffDelay)
	}
	
	// Verify hard threshold section
	if config.Hard == nil {
		t.Fatal("Expected Hard section to be set, got nil")
	}
	if config.Hard.Threshold != 100.0 {
		t.Errorf("Expected hard.threshold 100.0, got %f", config.Hard.Threshold)
	}
	if config.Hard.Plugin != "file_action" {
		t.Errorf("Expected hard.plugin 'file_action', got %q", config.Hard.Plugin)
	}
	if config.Hard.Duration.Seconds() != 45 {
		t.Errorf("Expected hard.duration 45s, got %v", config.Hard.Duration)
	}
	if config.Hard.BackoffDelay.Seconds() != 120 {
		t.Errorf("Expected hard.backoff_delay 2m, got %v", config.Hard.BackoffDelay)
	}
	
	// Verify backward compatibility - old fields should also be populated
	if config.SoftThreshold == nil || *config.SoftThreshold != 80.0 {
		t.Errorf("Expected backward compatible soft_threshold 80.0, got %v", config.SoftThreshold)
	}
	if config.HardThreshold == nil || *config.HardThreshold != 100.0 {
		t.Errorf("Expected backward compatible hard_threshold 100.0, got %v", config.HardThreshold)
	}
	if config.SoftThresholdPlugin != "log_action" {
		t.Errorf("Expected backward compatible soft_threshold_plugin 'log_action', got %q", config.SoftThresholdPlugin)
	}
	if config.HardThresholdPlugin != "file_action" {
		t.Errorf("Expected backward compatible hard_threshold_plugin 'file_action', got %q", config.HardThresholdPlugin)
	}
}

func TestLegacyFlatThresholdConfig(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Clear any existing threshold env vars
	thresholdEnvVars := []string{
		"SOFT_THRESHOLD", "HARD_THRESHOLD",
		"SOFT_THRESHOLD_PLUGIN", "HARD_THRESHOLD_PLUGIN",
		"SOFT_DURATION", "HARD_DURATION",
		"SOFT_BACKOFF_DELAY", "HARD_BACKOFF_DELAY",
		"THRESHOLD_DURATION", "BACKOFF_DELAY",
	}
	savedEnvs := make(map[string]string)
	for _, key := range thresholdEnvVars {
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
	
	// Create a config file with legacy flat threshold structure
	configContent := `log_level = "debug"
metric_name = "test_metric"
threshold_operator = "greater_than"

# Legacy flat structure
soft_threshold = 70.0
hard_threshold = 90.0
soft_threshold_plugin = "log_action"
hard_threshold_plugin = "file_action"
threshold_duration = "20s"
backoff_delay = "30s"
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
	
	// Verify legacy fields still work
	if config.SoftThreshold == nil || *config.SoftThreshold != 70.0 {
		t.Errorf("Expected soft_threshold 70.0, got %v", config.SoftThreshold)
	}
	if config.HardThreshold == nil || *config.HardThreshold != 90.0 {
		t.Errorf("Expected hard_threshold 90.0, got %v", config.HardThreshold)
	}
	if config.SoftThresholdPlugin != "log_action" {
		t.Errorf("Expected soft_threshold_plugin 'log_action', got %q", config.SoftThresholdPlugin)
	}
	if config.HardThresholdPlugin != "file_action" {
		t.Errorf("Expected hard_threshold_plugin 'file_action', got %q", config.HardThresholdPlugin)
	}
	if config.ThresholdDuration.Seconds() != 20 {
		t.Errorf("Expected threshold_duration 20s, got %v", config.ThresholdDuration)
	}
	if config.BackoffDelay.Seconds() != 30 {
		t.Errorf("Expected backoff_delay 30s, got %v", config.BackoffDelay)
	}
	
	// Verify new nested structure also gets populated
	if config.Soft == nil {
		t.Fatal("Expected Soft section to be populated from legacy config, got nil")
	}
	if config.Soft.Threshold != 70.0 {
		t.Errorf("Expected soft.threshold 70.0 from migration, got %f", config.Soft.Threshold)
	}
	if config.Soft.Plugin != "log_action" {
		t.Errorf("Expected soft.plugin 'log_action' from migration, got %q", config.Soft.Plugin)
	}
	if config.Soft.Duration.Seconds() != 20 {
		t.Errorf("Expected soft.duration 20s from migration, got %v", config.Soft.Duration)
	}
	if config.Soft.BackoffDelay.Seconds() != 30 {
		t.Errorf("Expected soft.backoff_delay 30s from migration, got %v", config.Soft.BackoffDelay)
	}
	
	if config.Hard == nil {
		t.Fatal("Expected Hard section to be populated from legacy config, got nil")
	}
	if config.Hard.Threshold != 90.0 {
		t.Errorf("Expected hard.threshold 90.0 from migration, got %f", config.Hard.Threshold)
	}
	if config.Hard.Plugin != "file_action" {
		t.Errorf("Expected hard.plugin 'file_action' from migration, got %q", config.Hard.Plugin)
	}
	if config.Hard.Duration.Seconds() != 20 {
		t.Errorf("Expected hard.duration 20s from migration, got %v", config.Hard.Duration)
	}
	if config.Hard.BackoffDelay.Seconds() != 30 {
		t.Errorf("Expected hard.backoff_delay 30s from migration, got %v", config.Hard.BackoffDelay)
	}
}

func TestEnvironmentVariableThresholdConfig(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Save original env vars and set test values
	thresholdEnvVars := map[string]string{
		"SOFT_THRESHOLD":      "85.5",
		"SOFT_DURATION":       "35s",
		"SOFT_BACKOFF_DELAY":  "90s",
		"HARD_THRESHOLD":      "95.5",
		"HARD_DURATION":       "40s",
		"HARD_BACKOFF_DELAY":  "120s",
		"THRESHOLD_OPERATOR":  "less_than",
	}
	savedEnvs := make(map[string]string)
	for key := range thresholdEnvVars {
		savedEnvs[key] = os.Getenv(key)
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
	
	// Set test env vars
	for key, value := range thresholdEnvVars {
		os.Setenv(key, value)
	}

	// Create a temporary directory without a config file
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	
	// Load config (should use env vars)
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	// Verify threshold values from environment
	if config.SoftThreshold == nil || *config.SoftThreshold != 85.5 {
		t.Errorf("Expected soft_threshold 85.5 from env, got %v", config.SoftThreshold)
	}
	if config.HardThreshold == nil || *config.HardThreshold != 95.5 {
		t.Errorf("Expected hard_threshold 95.5 from env, got %v", config.HardThreshold)
	}
	if config.ThresholdOperator != "less_than" {
		t.Errorf("Expected threshold_operator 'less_than' from env, got %q", config.ThresholdOperator)
	}
}
