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

// ThresholdSection holds configuration for a single threshold (soft or hard)
type ThresholdSection struct {
	Threshold    float64       `mapstructure:"threshold"`
	Plugin       string        `mapstructure:"plugin"`
	Duration     time.Duration `mapstructure:"duration"`
	BackoffDelay time.Duration `mapstructure:"backoff_delay"`
}

// Config holds all configuration for the application
type Config struct {
	// Logging
	LogLevel string `mapstructure:"log_level"`

	// Metric configuration
	MetricName   string `mapstructure:"metric_name"`
	LabelFilters string `mapstructure:"label_filters"`

	// Threshold configuration (new nested structure)
	ThresholdOperator string            `mapstructure:"threshold_operator"`
	Soft              *ThresholdSection `mapstructure:"soft"`
	Hard              *ThresholdSection `mapstructure:"hard"`

	// Deprecated: Flat threshold configuration (backward compatibility)
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

// syncStringWithDefault syncs a string field between nested and flat config with a default value
// If one has the default and the other doesn't, copy from the non-default one
// If both differ and nested is not default, prefer nested (new structure)
func syncStringWithDefault(nested *string, flat *string, defaultValue string) {
	if *nested == defaultValue && *flat != defaultValue {
		*nested = *flat
	} else if *flat == defaultValue && *nested != defaultValue {
		*flat = *nested
	} else if *flat != *nested {
		*flat = *nested
	}
}

// syncInt64WithDefault syncs an int64 field between nested and flat config with a default value
func syncInt64WithDefault(nested *int64, flat *int64, defaultValue int64) {
	if *nested == defaultValue && *flat != defaultValue {
		*nested = *flat
	} else if *flat == defaultValue && *nested != defaultValue {
		*flat = *nested
	} else if *flat != *nested {
		*flat = *nested
	}
}

// syncStringField syncs a string field between nested and flat config (no defaults)
// Syncs non-empty values, preferring nested when both are set
func syncStringField(nested *string, flat *string) {
	if *nested == "" && *flat != "" {
		*nested = *flat
	} else if *flat == "" && *nested != "" {
		*flat = *nested
	} else if *flat != *nested && *nested != "" {
		*flat = *nested
	}
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
	
	// New nested structure for soft/hard thresholds
	v.BindEnv("soft.threshold", "SOFT_THRESHOLD")
	v.BindEnv("soft.plugin", "SOFT_THRESHOLD_PLUGIN")
	v.BindEnv("soft.duration", "SOFT_DURATION")
	v.BindEnv("soft.backoff_delay", "SOFT_BACKOFF_DELAY")
	
	v.BindEnv("hard.threshold", "HARD_THRESHOLD")
	v.BindEnv("hard.plugin", "HARD_THRESHOLD_PLUGIN")
	v.BindEnv("hard.duration", "HARD_DURATION")
	v.BindEnv("hard.backoff_delay", "HARD_BACKOFF_DELAY")
	
	// Old flat structure (backward compatibility)
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

	// Bind plugin-specific environment variables to both old and new structures
	// Note: The same environment variable names are used for both to maintain backward compatibility
	// After unmarshaling, the sync helper functions will reconcile any differences between the two structures
	
	// Bind to old flat structure (backward compatibility)
	v.BindEnv("file_action_dir", "FILE_ACTION_DIR")
	v.BindEnv("file_action_size", "FILE_ACTION_SIZE")
	v.BindEnv("efs_file_system_id", "EFS_FILE_SYSTEM_ID")
	v.BindEnv("efs_file_system_prometheus_label", "EFS_FILE_SYSTEM_PROMETHEUS_LABEL")
	v.BindEnv("aws_region", "AWS_REGION")

	// Bind to new nested structure (same environment variable names)
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
	// Sync string fields with defaults
	syncStringWithDefault(&config.Plugins.FileAction.Dir, &config.FileActionDir, "/tmp/metric-files")
	
	// Sync int64 fields with defaults
	syncInt64WithDefault(&config.Plugins.FileAction.Size, &config.FileActionSize, int64(1024*1024))
	
	// Sync string fields without defaults (just non-empty values)
	syncStringField(&config.Plugins.EFSEmergency.FileSystemID, &config.EFSFileSystemID)
	syncStringField(&config.Plugins.EFSEmergency.FileSystemPrometheusLabel, &config.EFSFileSystemPrometheusLabel)
	syncStringField(&config.Plugins.EFSEmergency.AWSRegion, &config.AWSRegion)

	// Handle backward compatibility for threshold configuration
	// If new soft section is not set but old fields are, migrate them
	if config.Soft == nil && (config.SoftThreshold != nil || config.SoftThresholdPlugin != "" || 
		config.ThresholdDuration > 0 || config.BackoffDelay > 0) {
		config.Soft = &ThresholdSection{}
		if config.SoftThreshold != nil {
			config.Soft.Threshold = *config.SoftThreshold
		}
		config.Soft.Plugin = config.SoftThresholdPlugin
		config.Soft.Duration = config.ThresholdDuration
		config.Soft.BackoffDelay = config.BackoffDelay
	}
	
	// If new hard section is not set but old fields are, migrate them
	if config.Hard == nil && (config.HardThreshold != nil || config.HardThresholdPlugin != "" ||
		config.ThresholdDuration > 0 || config.BackoffDelay > 0) {
		config.Hard = &ThresholdSection{}
		if config.HardThreshold != nil {
			config.Hard.Threshold = *config.HardThreshold
		}
		config.Hard.Plugin = config.HardThresholdPlugin
		config.Hard.Duration = config.ThresholdDuration
		config.Hard.BackoffDelay = config.BackoffDelay
	}
	
	// Sync back to old fields for backward compatibility in code
	if config.Soft != nil {
		if config.SoftThreshold == nil {
			config.SoftThreshold = &config.Soft.Threshold
		}
		if config.SoftThresholdPlugin == "" {
			config.SoftThresholdPlugin = config.Soft.Plugin
		}
		// Don't override duration/backoff if old fields already set
		if config.ThresholdDuration == 0 {
			config.ThresholdDuration = config.Soft.Duration
		}
		if config.BackoffDelay == 0 {
			config.BackoffDelay = config.Soft.BackoffDelay
		}
	}
	
	if config.Hard != nil {
		if config.HardThreshold == nil {
			config.HardThreshold = &config.Hard.Threshold
		}
		if config.HardThresholdPlugin == "" {
			config.HardThresholdPlugin = config.Hard.Plugin
		}
		// Duration and backoff delay are shared across soft/hard in new structure
		// but each section can override them if needed
		if config.Hard.Duration > 0 {
			// Hard has its own duration, use it
			// Note: we'll need to handle this in main.go
		}
	}

	return &config, nil
}
