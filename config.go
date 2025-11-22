package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// PluginConfig holds plugin-specific configuration
type PluginConfig struct {
	// File Action Plugin configuration
	FileAction struct {
		Dir  string `mapstructure:"dir"`
		Size int64  `mapstructure:"size"`
	} `mapstructure:"file_action"`

	// EFS Emergency Plugin configuration
	EFSEmergency struct {
		FileSystemID              string `mapstructure:"file_system_id"`
		FileSystemPrometheusLabel string `mapstructure:"file_system_prometheus_label"`
		AWSRegion                 string `mapstructure:"aws_region"`
	} `mapstructure:"efs_emergency"`
}

// Config holds all configuration for the application
type Config struct {
	// Logging
	LogLevel string `mapstructure:"log_level"`

	// Metric configuration
	MetricName   string `mapstructure:"metric_name"`
	LabelFilters string `mapstructure:"label_filters"`

	// Threshold configuration
	ThresholdOperator   string        `mapstructure:"threshold_operator"`
	SoftThreshold       *float64      `mapstructure:"soft_threshold"`
	HardThreshold       *float64      `mapstructure:"hard_threshold"`
	SoftThresholdPlugin string        `mapstructure:"soft_threshold_plugin"`
	HardThresholdPlugin string        `mapstructure:"hard_threshold_plugin"`
	ThresholdDuration   time.Duration `mapstructure:"threshold_duration"`
	BackoffDelay        time.Duration `mapstructure:"backoff_delay"`

	// Polling configuration
	PollingInterval time.Duration `mapstructure:"polling_interval"`

	// Prometheus configuration
	PrometheusEndpoint string `mapstructure:"prometheus_endpoint"`

	// Plugin configuration
	PluginDir string `mapstructure:"plugin_dir"`

	// Leader election configuration
	LeaderElectionEnabled       bool   `mapstructure:"leader_election_enabled"`
	LeaderElectionLockName      string `mapstructure:"leader_election_lock_name"`
	LeaderElectionLockNamespace string `mapstructure:"leader_election_lock_namespace"`

	// Missing value behavior
	MissingValueBehavior string `mapstructure:"missing_value_behavior"`

	// Plugin-specific configuration (new nested structure)
	Plugins PluginConfig `mapstructure:"plugins"`

	// Deprecated: Plugin-specific configuration (backward compatibility)
	FileActionDir                string `mapstructure:"file_action_dir"`
	FileActionSize               int64  `mapstructure:"file_action_size"`
	EFSFileSystemID              string `mapstructure:"efs_file_system_id"`
	EFSFileSystemPrometheusLabel string `mapstructure:"efs_file_system_prometheus_label"`
	AWSRegion                    string `mapstructure:"aws_region"`
}

// LoadConfig loads configuration from file and environment variables
// Environment variables take precedence over config file values
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set defaults for main configuration
	v.SetDefault("log_level", "info")
	v.SetDefault("polling_interval", "1s")
	v.SetDefault("prometheus_endpoint", "http://prometheus:9090")
	v.SetDefault("threshold_duration", "0s")
	v.SetDefault("backoff_delay", "0s")
	v.SetDefault("leader_election_enabled", true)
	v.SetDefault("leader_election_lock_name", "metric-reader-leader")
	v.SetDefault("leader_election_lock_namespace", "")
	v.SetDefault("missing_value_behavior", "zero")

	// Set defaults for plugin configuration (new nested structure)
	v.SetDefault("plugins.file_action.dir", "/tmp/metric-files")
	v.SetDefault("plugins.file_action.size", 1024*1024) // 1MB

	// Set defaults for backward compatibility (deprecated flat structure)
	v.SetDefault("file_action_dir", "/tmp/metric-files")
	v.SetDefault("file_action_size", 1024*1024) // 1MB

	// EFS Emergency Plugin defaults
	// Note: EFS_FILE_SYSTEM_ID and EFS_FILE_SYSTEM_PROMETHEUS_LABEL have no defaults
	// as at least one must be explicitly configured
	// AWS_REGION has no default as it's auto-detected by AWS SDK

	// Set config file name and search paths
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/metric-reader")

	// Read config file if it exists (it's optional)
	if err := v.ReadInConfig(); err != nil {
		var notFoundErr viper.ConfigFileNotFoundError
		if !errors.As(err, &notFoundErr) {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		log.Debug().Msg("no config file found, using environment variables and defaults")
	} else {
		log.Info().Str("config_file", v.ConfigFileUsed()).Msg("loaded config file")
	}

	// Bind environment variables
	// Environment variables take precedence over config file
	v.AutomaticEnv()
	// Bind each config key to its corresponding environment variable
	v.BindEnv("log_level", "LOG_LEVEL")
	v.BindEnv("metric_name", "METRIC_NAME")
	v.BindEnv("label_filters", "LABEL_FILTERS")
	v.BindEnv("threshold_operator", "THRESHOLD_OPERATOR")
	v.BindEnv("soft_threshold", "SOFT_THRESHOLD")
	v.BindEnv("hard_threshold", "HARD_THRESHOLD")
	v.BindEnv("soft_threshold_plugin", "SOFT_THRESHOLD_PLUGIN")
	v.BindEnv("hard_threshold_plugin", "HARD_THRESHOLD_PLUGIN")
	v.BindEnv("threshold_duration", "THRESHOLD_DURATION")
	v.BindEnv("backoff_delay", "BACKOFF_DELAY")
	v.BindEnv("polling_interval", "POLLING_INTERVAL")
	v.BindEnv("prometheus_endpoint", "PROMETHEUS_ENDPOINT")
	v.BindEnv("plugin_dir", "PLUGIN_DIR")
	v.BindEnv("leader_election_enabled", "LEADER_ELECTION_ENABLED")
	v.BindEnv("leader_election_lock_name", "LEADER_ELECTION_LOCK_NAME")
	v.BindEnv("leader_election_lock_namespace", "LEADER_ELECTION_LOCK_NAMESPACE")
	v.BindEnv("missing_value_behavior", "MISSING_VALUE_BEHAVIOR")

	// Bind plugin-specific environment variables (backward compatibility)
	v.BindEnv("file_action_dir", "FILE_ACTION_DIR")
	v.BindEnv("file_action_size", "FILE_ACTION_SIZE")
	v.BindEnv("efs_file_system_id", "EFS_FILE_SYSTEM_ID")
	v.BindEnv("efs_file_system_prometheus_label", "EFS_FILE_SYSTEM_PROMETHEUS_LABEL")
	v.BindEnv("aws_region", "AWS_REGION")

	// Bind plugin-specific environment variables (new nested structure)
	v.BindEnv("plugins.file_action.dir", "FILE_ACTION_DIR")
	v.BindEnv("plugins.file_action.size", "FILE_ACTION_SIZE")
	v.BindEnv("plugins.efs_emergency.file_system_id", "EFS_FILE_SYSTEM_ID")
	v.BindEnv("plugins.efs_emergency.file_system_prometheus_label", "EFS_FILE_SYSTEM_PROMETHEUS_LABEL")
	v.BindEnv("plugins.efs_emergency.aws_region", "AWS_REGION")

	// Parse config into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Handle backward compatibility bidirectionally
	// If old flat config has non-default values and new nested config is at defaults, copy from old to new
	// If new nested config has non-default values and old flat config is at defaults, copy from new to old
	
	// Default values for comparison
	defaultFileActionDir := "/tmp/metric-files"
	defaultFileActionSize := int64(1024 * 1024)
	
	// Sync FileAction.Dir
	if config.Plugins.FileAction.Dir == defaultFileActionDir && config.FileActionDir != defaultFileActionDir {
		// Old config has non-default value, use it
		config.Plugins.FileAction.Dir = config.FileActionDir
	} else if config.FileActionDir == defaultFileActionDir && config.Plugins.FileAction.Dir != defaultFileActionDir {
		// New config has non-default value, use it
		config.FileActionDir = config.Plugins.FileAction.Dir
	} else if config.FileActionDir != config.Plugins.FileAction.Dir {
		// Both have non-default values and they differ - prefer nested config (new structure)
		config.FileActionDir = config.Plugins.FileAction.Dir
	}
	
	// Sync FileAction.Size
	if config.Plugins.FileAction.Size == defaultFileActionSize && config.FileActionSize != defaultFileActionSize {
		// Old config has non-default value, use it
		config.Plugins.FileAction.Size = config.FileActionSize
	} else if config.FileActionSize == defaultFileActionSize && config.Plugins.FileAction.Size != defaultFileActionSize {
		// New config has non-default value, use it
		config.FileActionSize = config.Plugins.FileAction.Size
	} else if config.FileActionSize != config.Plugins.FileAction.Size {
		// Both have non-default values and they differ - prefer nested config (new structure)
		config.FileActionSize = config.Plugins.FileAction.Size
	}
	
	// Sync EFS Emergency config (no defaults to compare against, just sync non-empty values)
	if config.Plugins.EFSEmergency.FileSystemID == "" && config.EFSFileSystemID != "" {
		config.Plugins.EFSEmergency.FileSystemID = config.EFSFileSystemID
	} else if config.EFSFileSystemID == "" && config.Plugins.EFSEmergency.FileSystemID != "" {
		config.EFSFileSystemID = config.Plugins.EFSEmergency.FileSystemID
	} else if config.EFSFileSystemID != config.Plugins.EFSEmergency.FileSystemID && config.Plugins.EFSEmergency.FileSystemID != "" {
		// Both have values and they differ - prefer nested config (new structure)
		config.EFSFileSystemID = config.Plugins.EFSEmergency.FileSystemID
	}
	
	if config.Plugins.EFSEmergency.FileSystemPrometheusLabel == "" && config.EFSFileSystemPrometheusLabel != "" {
		config.Plugins.EFSEmergency.FileSystemPrometheusLabel = config.EFSFileSystemPrometheusLabel
	} else if config.EFSFileSystemPrometheusLabel == "" && config.Plugins.EFSEmergency.FileSystemPrometheusLabel != "" {
		config.EFSFileSystemPrometheusLabel = config.Plugins.EFSEmergency.FileSystemPrometheusLabel
	} else if config.EFSFileSystemPrometheusLabel != config.Plugins.EFSEmergency.FileSystemPrometheusLabel && config.Plugins.EFSEmergency.FileSystemPrometheusLabel != "" {
		// Both have values and they differ - prefer nested config (new structure)
		config.EFSFileSystemPrometheusLabel = config.Plugins.EFSEmergency.FileSystemPrometheusLabel
	}
	
	if config.Plugins.EFSEmergency.AWSRegion == "" && config.AWSRegion != "" {
		config.Plugins.EFSEmergency.AWSRegion = config.AWSRegion
	} else if config.AWSRegion == "" && config.Plugins.EFSEmergency.AWSRegion != "" {
		config.AWSRegion = config.Plugins.EFSEmergency.AWSRegion
	} else if config.AWSRegion != config.Plugins.EFSEmergency.AWSRegion && config.Plugins.EFSEmergency.AWSRegion != "" {
		// Both have values and they differ - prefer nested config (new structure)
		config.AWSRegion = config.Plugins.EFSEmergency.AWSRegion
	}

	return &config, nil
}
