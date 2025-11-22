package main

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type thresholdOperator string

const (
	thresholdOperatorGreaterThan thresholdOperator = "greater_than"
	thresholdOperatorLessThan    thresholdOperator = "less_than"
)

type threshold struct {
	value  float64
	plugin ActionPlugin
}

type thresholdConfig struct {
	operator      thresholdOperator
	softThreshold *threshold
	hardThreshold *threshold
}

func parseThresholdOperator(operatorStr string) (thresholdOperator, error) {
	switch operatorStr {
	case "greater_than":
		return thresholdOperatorGreaterThan, nil
	case "less_than":
		return thresholdOperatorLessThan, nil
	default:
		return "", fmt.Errorf("threshold operator must be 'greater_than' or 'less_than'")
	}
}

func isThresholdCrossed(operator thresholdOperator, value float64, threshold float64) bool {
	switch operator {
	case thresholdOperatorGreaterThan:
		return value > threshold
	case thresholdOperatorLessThan:
		return value < threshold
	default:
		return false
	}
}

func validateThresholdPlugin(pluginName string, thresholdValue *threshold, thresholdType string) {
	if pluginName != "" {
		if thresholdValue == nil {
			log.Fatal().Str("plugin", pluginName).Msgf("%s_THRESHOLD_PLUGIN specified but %s_THRESHOLD is not set", thresholdType, thresholdType)
		}
		plugin, ok := PluginRegistry[pluginName]
		if !ok {
			log.Fatal().Str("plugin", pluginName).Msgf("specified %s threshold plugin not found", thresholdType)
		}
		thresholdValue.plugin = plugin
	}
}

func formatThresholdString(operator thresholdOperator, value float64) string {
	return fmt.Sprintf("%s %.2f", operator, value)
}

func main() {
	// Root context for the process and leader election
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration from file and environment variables
	config, err := LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load configuration")
	}

	// Start (optional) leader election. If disabled or not possible the instance
	// assumes singleton behaviour and continues as leader.
	startLeaderElection(ctx, config)

	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Set log level from config
	switch config.LogLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "":
		// Default to info if not set
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	default:
		log.Fatal().Str("LOG_LEVEL", config.LogLevel).Msg("invalid LOG_LEVEL value")
	}

	// Get metric name from config
	metricName := config.MetricName
	if metricName == "" {
		log.Fatal().Msg("METRIC_NAME is required")
	}

	// Get label filters from config
	labelFilters := config.LabelFilters
	var query string
	if labelFilters != "" {
		query = fmt.Sprintf("%s{%s}", metricName, labelFilters)
	} else {
		query = metricName
	}

	// Get threshold configuration from config
	var thresholdCfg *thresholdConfig
	
	if config.ThresholdOperator != "" && (config.SoftThreshold != 0 || config.HardThreshold != 0) {
		operator, err := parseThresholdOperator(config.ThresholdOperator)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid THRESHOLD_OPERATOR value")
		}
		
		thresholdCfg = &thresholdConfig{
			operator: operator,
		}
		
		// Parse soft threshold if provided
		if config.SoftThreshold != 0 {
			thresholdCfg.softThreshold = &threshold{
				value: config.SoftThreshold,
			}
		}
		
		// Parse hard threshold if provided
		if config.HardThreshold != 0 {
			thresholdCfg.hardThreshold = &threshold{
				value: config.HardThreshold,
			}
		}
	}

	// Get threshold duration from config
	thresholdDuration := config.ThresholdDuration

	// Get backoff delay from config
	backoffDelay := config.BackoffDelay

	// Get polling interval from config
	pollingInterval := config.PollingInterval

	// Get Prometheus endpoint from config
	prometheusEndpoint := config.PrometheusEndpoint

	// Get plugin directory from config
	pluginDir := config.PluginDir
	if pluginDir != "" {
		if err := LoadPluginsFromDirectory(pluginDir); err != nil {
			log.Error().Err(err).Msg("failed to load plugins")
		}
	}

	// Assign plugins to thresholds
	if thresholdCfg != nil {
		validateThresholdPlugin(config.SoftThresholdPlugin, thresholdCfg.softThreshold, "SOFT")
		validateThresholdPlugin(config.HardThresholdPlugin, thresholdCfg.hardThreshold, "HARD")
	}

	logEvent := log.Info().
		Str("metric_name", metricName).
		Str("prometheus_endpoint", prometheusEndpoint).
		Dur("polling_interval", pollingInterval).
		Dur("threshold_duration", thresholdDuration).
		Str("query", query)
	
	if thresholdCfg != nil {
		logEvent = logEvent.Str("threshold_operator", string(thresholdCfg.operator))
		if thresholdCfg.softThreshold != nil {
			logEvent = logEvent.Float64("soft_threshold", thresholdCfg.softThreshold.value)
			if thresholdCfg.softThreshold.plugin != nil {
				logEvent = logEvent.Str("soft_threshold_plugin", thresholdCfg.softThreshold.plugin.Name())
			}
		}
		if thresholdCfg.hardThreshold != nil {
			logEvent = logEvent.Float64("hard_threshold", thresholdCfg.hardThreshold.value)
			if thresholdCfg.hardThreshold.plugin != nil {
				logEvent = logEvent.Str("hard_threshold_plugin", thresholdCfg.hardThreshold.plugin.Name())
			}
		}
	}
	
	logEvent.Msg("initializing metric reader")

	// Create Prometheus client
	client, err := api.NewClient(api.Config{
		Address: prometheusEndpoint,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("error creating prometheus client")
	}

	v1api := v1.NewAPI(client)
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	log.Info().
		Str("query", query).
		Dur("polling_interval", pollingInterval).
		Msg("starting metric reader")

	var softThresholdStartTime time.Time
	var softThresholdActive bool
	var hardThresholdStartTime time.Time
	var hardThresholdActive bool
	var softBackoffUntil time.Time
	var hardBackoffUntil time.Time

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		result, warnings, err := v1api.Query(ctx, query, time.Now())
		cancel()

		if err != nil {
			log.Error().
				Err(err).
				Str("query", query).
				Msgf("error querying prometheus: %v", err)
			continue
		}

		if len(warnings) > 0 {
			log.Warn().
				Strs("warnings", warnings).
				Str("query", query).
				Msgf("prometheus query warnings: %v", warnings)
		}

		if result.Type() == model.ValVector {
			vector := result.(model.Vector)
			if len(vector) > 0 {
				value := float64(vector[0].Value)

				log.Debug().
					Str("query", query).
					Float64("value", value).
					Msg("reading metric value")

				// Process threshold configuration if set
				if thresholdCfg != nil {
					// Process soft threshold
					if thresholdCfg.softThreshold != nil {
						// Skip check if in backoff period
						if !softBackoffUntil.IsZero() && time.Now().Before(softBackoffUntil) {
							log.Debug().
								Time("soft_backoff_until", softBackoffUntil).
								Msg("skipping soft threshold check - in backoff period")
						} else {
							// Check if soft threshold is crossed
							softCrossed := isThresholdCrossed(thresholdCfg.operator, value, thresholdCfg.softThreshold.value)

							if softCrossed {
								if !softThresholdActive {
									// Start monitoring soft threshold duration
									softThresholdStartTime = time.Now()
									softThresholdActive = true
									log.Info().
										Str("query", query).
										Float64("value", value).
										Float64("soft_threshold", thresholdCfg.softThreshold.value).
										Str("operator", string(thresholdCfg.operator)).
										Msg("soft threshold crossed")
								} else if time.Since(softThresholdStartTime) >= thresholdDuration {
									// Soft threshold exceeded for required duration
									log.Warn().
										Str("query", query).
										Float64("value", value).
										Float64("soft_threshold", thresholdCfg.softThreshold.value).
										Dur("duration", time.Since(softThresholdStartTime)).
										Msg("soft threshold exceeded for specified duration")

									// Execute plugin action if configured and this replica is the current leader
									if thresholdCfg.softThreshold.plugin != nil && IsLeader() {
										thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.softThreshold.value)
										if err := thresholdCfg.softThreshold.plugin.Execute(ctx, metricName, value, thresholdStr, time.Since(softThresholdStartTime)); err != nil {
											log.Error().
												Err(err).
												Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
												Msg("failed to execute soft threshold plugin action")
										} else {
											// Set backoff period after successful action
											if backoffDelay > 0 {
												softBackoffUntil = time.Now().Add(backoffDelay)
												// reset threshold start time
												softThresholdStartTime = time.Time{}
												softThresholdActive = false
												log.Info().
													Str("query", query).
													Time("soft_backoff_until", softBackoffUntil).
													Msg("entering soft threshold backoff period after action")
											}
										}
									}
								}
							} else if softThresholdActive {
								// Soft threshold no longer crossed
								softThresholdActive = false
								softThresholdStartTime = time.Time{}
								log.Info().
									Str("query", query).
									Float64("value", value).
									Float64("soft_threshold", thresholdCfg.softThreshold.value).
									Msg("soft threshold no longer crossed")
							}
						}
					}

					// Process hard threshold
					if thresholdCfg.hardThreshold != nil {
						// Skip check if in backoff period
						if !hardBackoffUntil.IsZero() && time.Now().Before(hardBackoffUntil) {
							log.Debug().
								Time("hard_backoff_until", hardBackoffUntil).
								Msg("skipping hard threshold check - in backoff period")
						} else {
							// Check if hard threshold is crossed
							hardCrossed := isThresholdCrossed(thresholdCfg.operator, value, thresholdCfg.hardThreshold.value)

							if hardCrossed {
								if !hardThresholdActive {
									// Start monitoring hard threshold duration
									hardThresholdStartTime = time.Now()
									hardThresholdActive = true
									log.Info().
										Str("query", query).
										Float64("value", value).
										Float64("hard_threshold", thresholdCfg.hardThreshold.value).
										Str("operator", string(thresholdCfg.operator)).
										Msg("hard threshold crossed")
								} else if time.Since(hardThresholdStartTime) >= thresholdDuration {
									// Hard threshold exceeded for required duration
									log.Warn().
										Str("query", query).
										Float64("value", value).
										Float64("hard_threshold", thresholdCfg.hardThreshold.value).
										Dur("duration", time.Since(hardThresholdStartTime)).
										Msg("hard threshold exceeded for specified duration")

									// Execute plugin action if configured and this replica is the current leader
									if thresholdCfg.hardThreshold.plugin != nil && IsLeader() {
										thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.hardThreshold.value)
										if err := thresholdCfg.hardThreshold.plugin.Execute(ctx, metricName, value, thresholdStr, time.Since(hardThresholdStartTime)); err != nil {
											log.Error().
												Err(err).
												Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
												Msg("failed to execute hard threshold plugin action")
										} else {
											// Set backoff period after successful action
											if backoffDelay > 0 {
												hardBackoffUntil = time.Now().Add(backoffDelay)
												// reset threshold start time
												hardThresholdStartTime = time.Time{}
												hardThresholdActive = false
												log.Info().
													Str("query", query).
													Time("hard_backoff_until", hardBackoffUntil).
													Msg("entering hard threshold backoff period after action")
											}
										}
									}
								}
							} else if hardThresholdActive {
								// Hard threshold no longer crossed
								hardThresholdActive = false
								hardThresholdStartTime = time.Time{}
								log.Info().
									Str("query", query).
									Float64("value", value).
									Float64("hard_threshold", thresholdCfg.hardThreshold.value).
									Msg("hard threshold no longer crossed")
							}
						}
					}
				}
			} else {
				log.Warn().
					Str("query", query).
					Msg("no data found for metric")
			}
		} else {
			log.Error().
				Str("query", query).
				Str("result_type", result.Type().String()).
				Msg("unexpected result type")
		}
	}
}
