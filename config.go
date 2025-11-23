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

	// Threshold configuration
	ThresholdOperator string            `mapstructure:"threshold_operator"`
	Soft              *ThresholdSection `mapstructure:"soft"`
	Hard              *ThresholdSection `mapstructure:"hard"`

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

	// Plugin-specific configuration
	Plugins PluginConfig `mapstructure:"plugins"`
}

// LoadConfig loads configuration from file and environment variables
// Environment variables take precedence over config file values
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set defaults for main configuration
	v.SetDefault("log_level", "info")
	v.SetDefault("polling_interval", "1s")
	v.SetDefault("prometheus_endpoint", "http://prometheus:9090")
	v.SetDefault("leader_election_enabled", true)
	v.SetDefault("leader_election_lock_name", "metric-reader-leader")
	v.SetDefault("leader_election_lock_namespace", "")
	v.SetDefault("missing_value_behavior", "zero")

	// Set defaults for plugin configuration
	v.SetDefault("plugins.file_action.dir", "/tmp/metric-files")
	v.SetDefault("plugins.file_action.size", 1024*1024) // 1MB

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
	
	// Threshold configuration
	v.BindEnv("soft.threshold", "SOFT_THRESHOLD")
	v.BindEnv("soft.plugin", "SOFT_THRESHOLD_PLUGIN")
	v.BindEnv("soft.duration", "SOFT_DURATION")
	v.BindEnv("soft.backoff_delay", "SOFT_BACKOFF_DELAY")
	
	v.BindEnv("hard.threshold", "HARD_THRESHOLD")
	v.BindEnv("hard.plugin", "HARD_THRESHOLD_PLUGIN")
	v.BindEnv("hard.duration", "HARD_DURATION")
	v.BindEnv("hard.backoff_delay", "HARD_BACKOFF_DELAY")
	
	v.BindEnv("polling_interval", "POLLING_INTERVAL")
	v.BindEnv("prometheus_endpoint", "PROMETHEUS_ENDPOINT")
	v.BindEnv("plugin_dir", "PLUGIN_DIR")
	v.BindEnv("leader_election_enabled", "LEADER_ELECTION_ENABLED")
	v.BindEnv("leader_election_lock_name", "LEADER_ELECTION_LOCK_NAME")
	v.BindEnv("leader_election_lock_namespace", "LEADER_ELECTION_LOCK_NAMESPACE")
	v.BindEnv("missing_value_behavior", "MISSING_VALUE_BEHAVIOR")

	// Plugin-specific configuration
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

	return &config, nil
}
