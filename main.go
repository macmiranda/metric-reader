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

type missingValueBehavior string

const (
	missingValueBehaviorLastValue      missingValueBehavior = "last_value"
	missingValueBehaviorZero           missingValueBehavior = "zero"
	missingValueBehaviorAssumeBreached missingValueBehavior = "assume_breached"
)

// thresholdState represents the current state in the threshold state machine
type thresholdState string

const (
	stateNotBreached          thresholdState = "NotBreached"
	stateSoftThresholdActive  thresholdState = "SoftThresholdActive"
	stateHardThresholdActive  thresholdState = "HardThresholdActive"
)

// stateData holds the data associated with the current state
type stateData struct {
	currentState           thresholdState
	softThresholdStartTime time.Time
	hardThresholdStartTime time.Time
	softBackoffUntil       time.Time
	hardBackoffUntil       time.Time
}

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

func parseMissingValueBehavior(behaviorStr string) (missingValueBehavior, error) {
	switch behaviorStr {
	case string(missingValueBehaviorLastValue):
		return missingValueBehaviorLastValue, nil
	case string(missingValueBehaviorZero):
		return missingValueBehaviorZero, nil
	case string(missingValueBehaviorAssumeBreached):
		return missingValueBehaviorAssumeBreached, nil
	default:
		return "", fmt.Errorf("missing value behavior must be 'last_value', 'zero', or 'assume_breached'")
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

// processThresholdStateMachine handles state transitions for the threshold state machine
func processThresholdStateMachine(
	state *stateData,
	thresholdCfg *thresholdConfig,
	value float64,
	softDuration time.Duration,
	softBackoffDelay time.Duration,
	hardDuration time.Duration,
	hardBackoffDelay time.Duration,
	metricName string,
	query string,
) {
	now := time.Now()
	
	// Check if thresholds are crossed
	softCrossed := false
	hardCrossed := false
	
	if thresholdCfg.softThreshold != nil {
		softCrossed = isThresholdCrossed(thresholdCfg.operator, value, thresholdCfg.softThreshold.value)
	}
	
	if thresholdCfg.hardThreshold != nil {
		hardCrossed = isThresholdCrossed(thresholdCfg.operator, value, thresholdCfg.hardThreshold.value)
	}
	
	log.Debug().
		Str("current_state", string(state.currentState)).
		Bool("soft_crossed", softCrossed).
		Bool("hard_crossed", hardCrossed).
		Float64("value", value).
		Msg("evaluating threshold state machine")
	
	// State machine transitions
	switch state.currentState {
	case stateNotBreached:
		// Transition: NotBreached -> SoftThresholdActive (when soft threshold crossed for duration)
		if softCrossed && thresholdCfg.softThreshold != nil {
			// Check if we're in backoff period
			if !state.softBackoffUntil.IsZero() && now.Before(state.softBackoffUntil) {
				log.Debug().
					Time("soft_backoff_until", state.softBackoffUntil).
					Msg("in soft threshold backoff period")
				return
			}
			
			// Start timing the threshold crossing
			if state.softThresholdStartTime.IsZero() {
				state.softThresholdStartTime = now
				log.Debug().
					Str("query", query).
					Float64("value", value).
					Float64("soft_threshold", thresholdCfg.softThreshold.value).
					Str("operator", string(thresholdCfg.operator)).
					Msg("soft threshold crossed, starting duration timer")
			} else if now.Sub(state.softThresholdStartTime) >= softDuration {
				// Duration exceeded, transition to SoftThresholdActive
				oldState := state.currentState
				state.currentState = stateSoftThresholdActive
				
				log.Info().
					Str("previous_state", string(oldState)).
					Str("new_state", string(state.currentState)).
					Float64("value", value).
					Float64("soft_threshold", thresholdCfg.softThreshold.value).
					Dur("duration", now.Sub(state.softThresholdStartTime)).
					Msg("state transition: entering soft threshold active state")
				
				// Execute soft threshold plugin
				if thresholdCfg.softThreshold.plugin != nil && IsLeader() {
					thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.softThreshold.value)
					
					log.Debug().
						Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
						Str("state", string(state.currentState)).
						Msg("executing soft threshold plugin")
					
					if err := thresholdCfg.softThreshold.plugin.Execute(context.Background(), metricName, value, thresholdStr, now.Sub(state.softThresholdStartTime)); err != nil {
						log.Error().
							Err(err).
							Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("failed to execute soft threshold plugin action")
					} else {
						log.Info().
							Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("soft threshold plugin executed successfully")
						
						// Set backoff period after successful action
						if softBackoffDelay > 0 {
							state.softBackoffUntil = now.Add(softBackoffDelay)
							log.Debug().
								Time("soft_backoff_until", state.softBackoffUntil).
								Dur("backoff_delay", softBackoffDelay).
								Msg("soft threshold backoff period started")
						}
					}
				}
			}
		} else if !softCrossed && !state.softThresholdStartTime.IsZero() {
			// Threshold no longer crossed before duration elapsed, reset timer
			log.Debug().
				Str("query", query).
				Msg("soft threshold no longer crossed before duration elapsed, resetting timer")
			state.softThresholdStartTime = time.Time{}
		}
		
	case stateSoftThresholdActive:
		// Transition: SoftThresholdActive -> NotBreached (when threshold no longer crossed)
		if !softCrossed {
			oldState := state.currentState
			state.currentState = stateNotBreached
			state.softThresholdStartTime = time.Time{}
			
			log.Info().
				Str("previous_state", string(oldState)).
				Str("new_state", string(state.currentState)).
				Float64("value", value).
				Float64("soft_threshold", thresholdCfg.softThreshold.value).
				Msg("state transition: threshold no longer crossed, returning to not breached")
			return
		}
		
		// Transition: SoftThresholdActive -> HardThresholdActive (when hard threshold crossed for duration)
		if hardCrossed && thresholdCfg.hardThreshold != nil {
			// Check if we're in backoff period
			if !state.hardBackoffUntil.IsZero() && now.Before(state.hardBackoffUntil) {
				log.Debug().
					Time("hard_backoff_until", state.hardBackoffUntil).
					Msg("in hard threshold backoff period")
				return
			}
			
			// Start timing the hard threshold crossing
			if state.hardThresholdStartTime.IsZero() {
				state.hardThresholdStartTime = now
				log.Debug().
					Str("query", query).
					Float64("value", value).
					Float64("hard_threshold", thresholdCfg.hardThreshold.value).
					Str("operator", string(thresholdCfg.operator)).
					Msg("hard threshold crossed, starting duration timer")
			} else if now.Sub(state.hardThresholdStartTime) >= hardDuration {
				// Duration exceeded, transition to HardThresholdActive
				oldState := state.currentState
				state.currentState = stateHardThresholdActive
				
				log.Info().
					Str("previous_state", string(oldState)).
					Str("new_state", string(state.currentState)).
					Float64("value", value).
					Float64("hard_threshold", thresholdCfg.hardThreshold.value).
					Dur("duration", now.Sub(state.hardThresholdStartTime)).
					Msg("state transition: entering hard threshold active state")
				
				// Execute hard threshold plugin
				if thresholdCfg.hardThreshold.plugin != nil && IsLeader() {
					thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.hardThreshold.value)
					
					log.Debug().
						Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
						Str("state", string(state.currentState)).
						Msg("executing hard threshold plugin")
					
					if err := thresholdCfg.hardThreshold.plugin.Execute(context.Background(), metricName, value, thresholdStr, now.Sub(state.hardThresholdStartTime)); err != nil {
						log.Error().
							Err(err).
							Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("failed to execute hard threshold plugin action")
					} else {
						log.Info().
							Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("hard threshold plugin executed successfully")
						
						// Set backoff period after successful action
						if hardBackoffDelay > 0 {
							state.hardBackoffUntil = now.Add(hardBackoffDelay)
							log.Debug().
								Time("hard_backoff_until", state.hardBackoffUntil).
								Dur("backoff_delay", hardBackoffDelay).
								Msg("hard threshold backoff period started")
						}
					}
				}
			}
		} else if !hardCrossed && !state.hardThresholdStartTime.IsZero() {
			// Hard threshold no longer crossed before duration elapsed, reset timer
			log.Debug().
				Str("query", query).
				Msg("hard threshold no longer crossed before duration elapsed, resetting timer")
			state.hardThresholdStartTime = time.Time{}
		}
		
		// Stay in SoftThresholdActive: Check if we can re-execute soft plugin after backoff
		if softCrossed && thresholdCfg.softThreshold != nil {
			if !state.softBackoffUntil.IsZero() && now.After(state.softBackoffUntil) {
				// Backoff period has passed, can re-execute
				log.Debug().
					Msg("soft threshold backoff period expired, can re-execute plugin")
				
				if thresholdCfg.softThreshold.plugin != nil && IsLeader() {
					thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.softThreshold.value)
					
					log.Debug().
						Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
						Str("state", string(state.currentState)).
						Msg("re-executing soft threshold plugin after backoff")
					
					if err := thresholdCfg.softThreshold.plugin.Execute(context.Background(), metricName, value, thresholdStr, time.Duration(0)); err != nil {
						log.Error().
							Err(err).
							Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("failed to re-execute soft threshold plugin action")
					} else {
						log.Info().
							Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("soft threshold plugin re-executed successfully after backoff")
						
						// Reset backoff
						if softBackoffDelay > 0 {
							state.softBackoffUntil = now.Add(softBackoffDelay)
							log.Debug().
								Time("soft_backoff_until", state.softBackoffUntil).
								Msg("soft threshold backoff period restarted")
						}
					}
				}
			}
		}
		
	case stateHardThresholdActive:
		// Transition: HardThresholdActive -> NotBreached (when threshold no longer crossed)
		if !hardCrossed && !softCrossed {
			oldState := state.currentState
			state.currentState = stateNotBreached
			state.softThresholdStartTime = time.Time{}
			state.hardThresholdStartTime = time.Time{}
			
			log.Info().
				Str("previous_state", string(oldState)).
				Str("new_state", string(state.currentState)).
				Float64("value", value).
				Msg("state transition: thresholds no longer crossed, returning to not breached")
			return
		}
		
		// If soft threshold is no longer crossed, return to NotBreached
		// (hard threshold requires soft to be active first per the state machine)
		if !softCrossed {
			oldState := state.currentState
			state.currentState = stateNotBreached
			state.softThresholdStartTime = time.Time{}
			state.hardThresholdStartTime = time.Time{}
			
			log.Info().
				Str("previous_state", string(oldState)).
				Str("new_state", string(state.currentState)).
				Float64("value", value).
				Msg("state transition: soft threshold no longer crossed, returning to not breached")
			return
		}
		
		// Stay in HardThresholdActive: Check if we can re-execute hard plugin after backoff
		if hardCrossed && thresholdCfg.hardThreshold != nil {
			if !state.hardBackoffUntil.IsZero() && now.After(state.hardBackoffUntil) {
				// Backoff period has passed, can re-execute
				log.Debug().
					Msg("hard threshold backoff period expired, can re-execute plugin")
				
				if thresholdCfg.hardThreshold.plugin != nil && IsLeader() {
					thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.hardThreshold.value)
					
					log.Debug().
						Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
						Str("state", string(state.currentState)).
						Msg("re-executing hard threshold plugin after backoff")
					
					if err := thresholdCfg.hardThreshold.plugin.Execute(context.Background(), metricName, value, thresholdStr, time.Duration(0)); err != nil {
						log.Error().
							Err(err).
							Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("failed to re-execute hard threshold plugin action")
					} else {
						log.Info().
							Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
							Str("state", string(state.currentState)).
							Msg("hard threshold plugin re-executed successfully after backoff")
						
						// Reset backoff
						if hardBackoffDelay > 0 {
							state.hardBackoffUntil = now.Add(hardBackoffDelay)
							log.Debug().
								Time("hard_backoff_until", state.hardBackoffUntil).
								Msg("hard threshold backoff period restarted")
						}
					}
				}
			}
		}
	}
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
	var softDuration, hardDuration time.Duration
	var softBackoffDelay, hardBackoffDelay time.Duration

	if config.ThresholdOperator != "" && (config.Soft != nil || config.Hard != nil) {
		operator, err := parseThresholdOperator(config.ThresholdOperator)
		if err != nil {
			log.Fatal().Err(err).Msg("invalid THRESHOLD_OPERATOR value")
		}

		thresholdCfg = &thresholdConfig{
			operator: operator,
		}

		// Parse soft threshold if provided
		if config.Soft != nil {
			thresholdCfg.softThreshold = &threshold{
				value: config.Soft.Threshold,
			}
			softDuration = config.Soft.Duration
			softBackoffDelay = config.Soft.BackoffDelay
		}

		// Parse hard threshold if provided
		if config.Hard != nil {
			thresholdCfg.hardThreshold = &threshold{
				value: config.Hard.Threshold,
			}
			hardDuration = config.Hard.Duration
			hardBackoffDelay = config.Hard.BackoffDelay
		}
	}

	// Get polling interval from config
	pollingInterval := config.PollingInterval

	// Get Prometheus endpoint from config
	prometheusEndpoint := config.PrometheusEndpoint

	// Get missing value behavior from config
	missingValueBehavior, err := parseMissingValueBehavior(config.MissingValueBehavior)
	if err != nil {
		log.Fatal().Err(err).Str("MISSING_VALUE_BEHAVIOR", config.MissingValueBehavior).Msg("invalid MISSING_VALUE_BEHAVIOR value")
	}

	// Determine which plugins are needed
	requiredPlugins := make(map[string]bool)
	if config.Soft != nil && config.Soft.Plugin != "" {
		requiredPlugins[config.Soft.Plugin] = true
	}
	if config.Hard != nil && config.Hard.Plugin != "" {
		requiredPlugins[config.Hard.Plugin] = true
	}

	// Get plugin directory from config and load only required plugins
	pluginDir := config.PluginDir
	if pluginDir != "" && len(requiredPlugins) > 0 {
		if err := LoadRequiredPlugins(pluginDir, requiredPlugins); err != nil {
			log.Fatal().Err(err).Msg("failed to load required plugins")
		}
	}

	// Assign plugins to thresholds and validate configuration
	if thresholdCfg != nil {
		if config.Soft != nil {
			validateThresholdPlugin(config.Soft.Plugin, thresholdCfg.softThreshold, "SOFT")
		}
		if config.Hard != nil {
			validateThresholdPlugin(config.Hard.Plugin, thresholdCfg.hardThreshold, "HARD")
		}
	}

	logEvent := log.Info().
		Str("metric_name", metricName).
		Str("prometheus_endpoint", prometheusEndpoint).
		Dur("polling_interval", pollingInterval).
		Str("query", query).
		Str("missing_value_behavior", string(missingValueBehavior))

	if thresholdCfg != nil {
		logEvent = logEvent.Str("threshold_operator", string(thresholdCfg.operator))
		if thresholdCfg.softThreshold != nil {
			logEvent = logEvent.Float64("soft_threshold", thresholdCfg.softThreshold.value).
				Dur("soft_duration", softDuration).
				Dur("soft_backoff_delay", softBackoffDelay)
			if thresholdCfg.softThreshold.plugin != nil {
				logEvent = logEvent.Str("soft_threshold_plugin", thresholdCfg.softThreshold.plugin.Name())
			}
		}
		if thresholdCfg.hardThreshold != nil {
			logEvent = logEvent.Float64("hard_threshold", thresholdCfg.hardThreshold.value).
				Dur("hard_duration", hardDuration).
				Dur("hard_backoff_delay", hardBackoffDelay)
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

	// Initialize state machine
	state := &stateData{
		currentState: stateNotBreached,
	}
	var lastValue float64
	var hasLastValue bool

	log.Debug().
		Str("state", string(state.currentState)).
		Msg("initialized threshold state machine")

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

			var value float64
			var valueFound bool

			if len(vector) > 0 {
				value = float64(vector[0].Value)
				valueFound = true

				log.Debug().
					Str("query", query).
					Float64("value", value).
					Msg("reading metric value")

				// Update last value for potential reuse
				lastValue = value
				hasLastValue = true
			} else {
				// Handle missing value based on configured behavior
				log.Warn().
					Str("query", query).
					Str("missing_value_behavior", string(missingValueBehavior)).
					Msg("no data found for metric")

				switch missingValueBehavior {
				case missingValueBehaviorLastValue:
					if hasLastValue {
						value = lastValue
						valueFound = true
						log.Info().
							Str("query", query).
							Float64("value", value).
							Msg("using last known value for missing metric")
					} else {
						log.Warn().
							Str("query", query).
							Msg("no last value available, skipping threshold check")
					}
				case missingValueBehaviorZero:
					value = 0
					valueFound = true
					log.Info().
						Str("query", query).
						Float64("value", value).
						Msg("using zero for missing metric")
				case missingValueBehaviorAssumeBreached:
					// Activate configured thresholds immediately when data is missing
					if thresholdCfg != nil {
						log.Warn().
							Str("query", query).
							Str("current_state", string(state.currentState)).
							Msg("assuming thresholds breached for missing metric")

						// For assume_breached, transition to active states respecting the state machine
						now := time.Now()
						
						// If we're in NotBreached and soft threshold is configured, start soft threshold
						if state.currentState == stateNotBreached && thresholdCfg.softThreshold != nil {
							if state.softBackoffUntil.IsZero() || now.After(state.softBackoffUntil) {
								state.softThresholdStartTime = now
								// Immediately transition to active state
								oldState := state.currentState
								state.currentState = stateSoftThresholdActive
								
								log.Info().
									Str("previous_state", string(oldState)).
									Str("new_state", string(state.currentState)).
									Str("reason", "assume_breached").
									Msg("state transition: assuming soft threshold breached due to missing data")
								
								// Execute soft plugin
								if thresholdCfg.softThreshold.plugin != nil && IsLeader() {
									thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.softThreshold.value)
									log.Debug().
										Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
										Msg("executing soft threshold plugin due to assume_breached")
									
									if err := thresholdCfg.softThreshold.plugin.Execute(ctx, metricName, 0, thresholdStr, time.Duration(0)); err != nil {
										log.Error().
											Err(err).
											Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
											Msg("failed to execute soft threshold plugin for assume_breached")
									} else {
										log.Info().
											Str("plugin", thresholdCfg.softThreshold.plugin.Name()).
											Msg("soft threshold plugin executed for assume_breached")
										if softBackoffDelay > 0 {
											state.softBackoffUntil = now.Add(softBackoffDelay)
										}
									}
								}
							} else {
								log.Debug().
									Time("soft_backoff_until", state.softBackoffUntil).
									Msg("skipping soft threshold activation - in backoff period")
							}
						}

						// If in SoftThresholdActive and hard threshold is configured, transition to hard
						if state.currentState == stateSoftThresholdActive && thresholdCfg.hardThreshold != nil {
							if state.hardBackoffUntil.IsZero() || now.After(state.hardBackoffUntil) {
								state.hardThresholdStartTime = now
								oldState := state.currentState
								state.currentState = stateHardThresholdActive
								
								log.Info().
									Str("previous_state", string(oldState)).
									Str("new_state", string(state.currentState)).
									Str("reason", "assume_breached").
									Msg("state transition: assuming hard threshold breached due to missing data")
								
								// Execute hard plugin
								if thresholdCfg.hardThreshold.plugin != nil && IsLeader() {
									thresholdStr := formatThresholdString(thresholdCfg.operator, thresholdCfg.hardThreshold.value)
									log.Debug().
										Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
										Msg("executing hard threshold plugin due to assume_breached")
									
									if err := thresholdCfg.hardThreshold.plugin.Execute(ctx, metricName, 0, thresholdStr, time.Duration(0)); err != nil {
										log.Error().
											Err(err).
											Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
											Msg("failed to execute hard threshold plugin for assume_breached")
									} else {
										log.Info().
											Str("plugin", thresholdCfg.hardThreshold.plugin.Name()).
											Msg("hard threshold plugin executed for assume_breached")
										if hardBackoffDelay > 0 {
											state.hardBackoffUntil = now.Add(hardBackoffDelay)
										}
									}
								}
							} else {
								log.Debug().
									Time("hard_backoff_until", state.hardBackoffUntil).
									Msg("skipping hard threshold activation - in backoff period")
							}
						}
					}
					// Don't process thresholds normally for assume_breached
					valueFound = false
				}
			}

			// Process threshold configuration if set and we have a value to check
			if valueFound && thresholdCfg != nil {
				processThresholdStateMachine(state, thresholdCfg, value, softDuration, softBackoffDelay, hardDuration, hardBackoffDelay, metricName, query)
			}
		} else {
			log.Error().
				Str("query", query).
				Str("result_type", result.Type().String()).
				Msg("unexpected result type")
		}
	}
}
