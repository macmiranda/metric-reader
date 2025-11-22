package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type thresholdOperator string

const (
	thresholdOperatorGreaterThan thresholdOperator = ">"
	thresholdOperatorLessThan    thresholdOperator = "<"
)

type threshold struct {
	operator thresholdOperator
	value    float64
}

func parseThreshold(thresholdStr string) (*threshold, error) {
	if len(thresholdStr) < 2 {
		return nil, fmt.Errorf("invalid threshold format")
	}

	operator := thresholdStr[:1]
	if operator != ">" && operator != "<" {
		return nil, fmt.Errorf("threshold operator must be > or <")
	}

	value, err := strconv.ParseFloat(thresholdStr[1:], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid threshold value: %v", err)
	}

	return &threshold{
		operator: thresholdOperator(operator),
		value:    value,
	}, nil
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

	// Get threshold from config
	thresholdStr := config.Threshold
	var threshold *threshold
	if thresholdStr != "" {
		var err error
		threshold, err = parseThreshold(thresholdStr)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid THRESHOLD value")
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

	// Get action plugin name from config
	actionPluginName := config.ActionPlugin
	var actionPlugin ActionPlugin
	if actionPluginName != "" {
		var ok bool
		actionPlugin, ok = PluginRegistry[actionPluginName]
		if !ok {
			log.Fatal().Str("plugin", actionPluginName).Msg("specified action plugin not found")
		}
	}

	log.Info().
		Str("metric_name", metricName).
		Str("prometheus_endpoint", prometheusEndpoint).
		Dur("polling_interval", pollingInterval).
		Interface("threshold", threshold).
		Dur("threshold_duration", thresholdDuration).
		Str("action_plugin", actionPluginName).
		Str("query", query).
		Msg("initializing metric reader")

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

	var thresholdStartTime time.Time
	var thresholdActive bool
	var backoffUntil time.Time

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
					Time("threshold_start_time", thresholdStartTime).
					Dur("threshold_duration", time.Since(thresholdStartTime)).
					Time("backoff_until", backoffUntil).
					Msg("reading metric value")

				// Skip threshold checks if in backoff period
				if !backoffUntil.IsZero() && time.Now().Before(backoffUntil) {
					continue
				}

				// Process threshold if configured
				if threshold != nil {
					// Check if threshold is crossed
					thresholdCrossed := false
					if threshold.operator == thresholdOperatorGreaterThan && value > threshold.value {
						thresholdCrossed = true
					} else if threshold.operator == thresholdOperatorLessThan && value < threshold.value {
						thresholdCrossed = true
					}

					// Handle threshold state
					if thresholdCrossed {
						if !thresholdActive {
							// Start monitoring threshold duration
							thresholdStartTime = time.Now()
							thresholdActive = true
							log.Info().
								Str("query", query).
								Float64("value", value).
								Str("threshold", thresholdStr).
								Msg("threshold crossed")
						} else if time.Since(thresholdStartTime) >= thresholdDuration {
							// Threshold exceeded for required duration
							log.Warn().
								Str("query", query).
								Float64("value", value).
								Str("threshold", thresholdStr).
								Dur("duration", time.Since(thresholdStartTime)).
								Msg("threshold exceeded for specified duration")

							// Execute plugin action if configured and this replica is the current leader
							if actionPlugin != nil && IsLeader() {
								if err := actionPlugin.Execute(ctx, metricName, value, thresholdStr, time.Since(thresholdStartTime)); err != nil {
									log.Error().
										Err(err).
										Str("plugin", actionPlugin.Name()).
										Msg("failed to execute plugin action")
								} else {
									// Set backoff period after successful action
									if backoffDelay > 0 {
										backoffUntil = time.Now().Add(backoffDelay)
										// reset threshold start time
										thresholdStartTime = time.Time{}
										thresholdActive = false
										log.Info().
											Str("query", query).
											Time("backoff_until", backoffUntil).
											Msg("entering backoff period after action")
									}
								}
							}
						}
					} else if thresholdActive {
						// Threshold no longer crossed
						thresholdActive = false
						thresholdStartTime = time.Time{}
						thresholdCrossed = false
						log.Info().
							Str("query", query).
							Float64("value", value).
							Str("threshold", thresholdStr).
							Dur("duration", time.Since(thresholdStartTime)).
							Msg("threshold no longer crossed")
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
