package main

import (
	"context"
	"fmt"
	"os"
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
	greaterThan thresholdOperator = ">"
	lessThan    thresholdOperator = "<"
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

	// Start (optional) leader election. If disabled or not possible the instance
	// assumes singleton behaviour and continues as leader.
	startLeaderElection(ctx)

	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Set log level from environment
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		switch level {
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		case "info":
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		case "warn":
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		default:
			log.Fatal().Str("LOG_LEVEL", level).Msg("invalid LOG_LEVEL value")
		}
	}

	// Get metric name from environment variable
	metricName := os.Getenv("METRIC_NAME")
	if metricName == "" {
		log.Fatal().Msg("METRIC_NAME environment variable is required")
	}

	// Get label filters from environment variable
	labelFilters := os.Getenv("LABEL_FILTERS")
	var query string
	if labelFilters != "" {
		query = fmt.Sprintf("%s{%s}", metricName, labelFilters)
	} else {
		query = metricName
	}

	// Get threshold from environment variable
	thresholdStr := os.Getenv("THRESHOLD")
	var threshold *threshold
	if thresholdStr != "" {
		var err error
		threshold, err = parseThreshold(thresholdStr)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid THRESHOLD value")
		}
	}

	// Get threshold duration from environment variable
	thresholdDuration := 0 * time.Second
	if durationStr := os.Getenv("THRESHOLD_DURATION"); durationStr != "" {
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid THRESHOLD_DURATION value")
		}
		thresholdDuration = duration
	}

	// Get backoff delay from environment variable
	backoffDelay := 0 * time.Second
	if delayStr := os.Getenv("BACKOFF_DELAY"); delayStr != "" {
		delay, err := time.ParseDuration(delayStr)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid BACKOFF_DELAY value")
		}
		backoffDelay = delay
	}

	// Get polling interval from environment variable, default to 1 second
	pollingInterval := 1 * time.Second
	if intervalStr := os.Getenv("POLLING_INTERVAL"); intervalStr != "" {
		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid POLLING_INTERVAL value, must be a valid duration (e.g. 15s, 1m, 1h)")
		}
		pollingInterval = interval
	}

	// Get Prometheus endpoint from environment variable, default to local Prometheus
	prometheusEndpoint := os.Getenv("PROMETHEUS_ENDPOINT")
	if prometheusEndpoint == "" {
		prometheusEndpoint = "http://prometheus:9090"
	}

	// Get plugin directory from environment variable
	pluginDir := os.Getenv("PLUGIN_DIR")
	if pluginDir != "" {
		if err := LoadPluginsFromDirectory(pluginDir); err != nil {
			log.Error().Err(err).Msg("failed to load plugins")
		}
	}

	// Get action plugin name from environment variable
	actionPluginName := os.Getenv("ACTION_PLUGIN")
	var actionPlugin ActionPlugin
	if actionPluginName != "" {
		var ok bool
		actionPlugin, ok = PluginRegistry[actionPluginName]
		if !ok {
			log.Fatal().Str("plugin", actionPluginName).Msg("specified action plugin not found")
		}
	}

	// Get no metric behavior from environment variable, default to "zero"
	noMetricBehavior := os.Getenv("NO_METRIC_BEHAVIOR")
	if noMetricBehavior == "" {
		noMetricBehavior = "zero"
	}

	switch noMetricBehavior {
	case "last_value":
		log.Info().Msg("no metric behavior set to last_value")
	case "zero":
		log.Info().Msg("no metric behavior set to zero")
	case "assume_breached":
		log.Info().Msg("no metric behavior set to assume_breached")
	default:
		log.Fatal().Str("NO_METRIC_BEHAVIOR", noMetricBehavior).Msg("invalid NO_METRIC_BEHAVIOR value")
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

	var thresholdStartTime, backoffUntil time.Time
	var thresholdActive, thresholdCrossed bool
	var value, lastValue float64
	for range ticker.C {
		// Reset thresholdCrossed at the beginning of each iteration
		thresholdCrossed = false

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
				value, lastValue = float64(vector[0].Value), float64(vector[0].Value)
				log.Debug().
					Str("query", query).
					Float64("value", value).
					Time("threshold_start_time", thresholdStartTime).
					Dur("threshold_duration", time.Since(thresholdStartTime)).
					Time("backoff_until", backoffUntil).
					Msg("reading metric value")
			} else {
				log.Warn().
					Str("query", query).
					Msg("no data found for metric")
				switch noMetricBehavior {
				case "last_value":
					// Only use last value if we have one, otherwise skip this iteration
					if lastValue != 0 || thresholdActive {
						value = lastValue
						log.Info().Msg("using last value")
					} else {
						log.Info().Msg("no last value available, skipping iteration")
						continue
					}
				case "zero":
					value = 0
					log.Info().Msg("setting to zero")
				case "assume_breached":
					// Only set thresholdStartTime if not already active
					if !thresholdActive {
						thresholdStartTime = time.Now()
						thresholdActive = true
					}
					thresholdCrossed = true
					log.Info().Msg("assuming threshold is breached")
				}
			}
			// Skip threshold checks if in backoff period
			if !backoffUntil.IsZero() && time.Now().Before(backoffUntil) {
				continue
			}

			// Process threshold if configured
			if threshold != nil {
				// Check if threshold is crossed
				if threshold.operator == greaterThan && value > threshold.value {
					thresholdCrossed = true
				} else if threshold.operator == lessThan && value < threshold.value {
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
					duration := time.Since(thresholdStartTime)
					thresholdStartTime = time.Time{}
					thresholdCrossed = false
					log.Info().
						Str("query", query).
						Float64("value", value).
						Str("threshold", thresholdStr).
						Dur("duration", duration).
						Msg("threshold no longer crossed")
				}
			}
		} else {
			log.Error().
				Str("query", query).
				Str("result_type", result.Type().String()).
				Msg("unexpected result type")
		}
	}
}
