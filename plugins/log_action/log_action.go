package main

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// LogActionPlugin is a simple plugin that logs threshold events
type LogActionPlugin struct{}

// Execute implements the ActionPlugin interface
func (p *LogActionPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
	log.Info().
		Str("metric_name", metricName).
		Float64("value", value).
		Str("threshold", threshold).
		Dur("duration", duration).
		Msg("threshold action executed")
	return nil
}

// Name implements the ActionPlugin interface
func (p *LogActionPlugin) Name() string {
	return "log_action"
}

// ValidateConfig implements the ActionPlugin interface
func (p *LogActionPlugin) ValidateConfig() error {
	// Log action plugin has no required configuration
	return nil
}

// Plugin is the exported plugin symbol
var Plugin LogActionPlugin
