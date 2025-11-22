package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	// Logging
	LogLevel string `mapstructure:"log_level"`

	// Metric configuration
	MetricName   string `mapstructure:"metric_name"`
	LabelFilters string `mapstructure:"label_filters"`

	// Threshold configuration
	ThresholdOperator       string        `mapstructure:"threshold_operator"`
	SoftThreshold           float64       `mapstructure:"soft_threshold"`
	HardThreshold           float64       `mapstructure:"hard_threshold"`
	SoftThresholdPlugin     string        `mapstructure:"soft_threshold_plugin"`
	HardThresholdPlugin     string        `mapstructure:"hard_threshold_plugin"`
	ThresholdDuration       time.Duration `mapstructure:"threshold_duration"`
	BackoffDelay            time.Duration `mapstructure:"backoff_delay"`

	// Polling configuration
	PollingInterval time.Duration `mapstructure:"polling_interval"`

	// Prometheus configuration
	PrometheusEndpoint string `mapstructure:"prometheus_endpoint"`

	// Plugin configuration
	PluginDir string `mapstructure:"plugin_dir"`

	// Leader election configuration
	LeaderElectionEnabled  bool   `mapstructure:"leader_election_enabled"`
	LeaderElectionLockName string `mapstructure:"leader_election_lock_name"`
	PodNamespace           string `mapstructure:"pod_namespace"`

	// Plugin-specific configuration
	FileActionDir  string `mapstructure:"file_action_dir"`
	FileActionSize int64  `mapstructure:"file_action_size"`
}

// LoadConfig loads configuration from file and environment variables
// Environment variables take precedence over config file values
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("polling_interval", "1s")
	v.SetDefault("prometheus_endpoint", "http://prometheus:9090")
	v.SetDefault("threshold_duration", "0s")
	v.SetDefault("backoff_delay", "0s")
	v.SetDefault("leader_election_enabled", true)
	v.SetDefault("leader_election_lock_name", "metric-reader-leader")
	v.SetDefault("pod_namespace", "default")
	v.SetDefault("file_action_dir", "/tmp/metric-files")
	v.SetDefault("file_action_size", 1024*1024) // 1MB

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
	v.BindEnv("pod_namespace", "POD_NAMESPACE")
	v.BindEnv("file_action_dir", "FILE_ACTION_DIR")
	v.BindEnv("file_action_size", "FILE_ACTION_SIZE")

	// Parse config into struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}
