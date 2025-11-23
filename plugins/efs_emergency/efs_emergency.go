package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

// EFSEmergencyPlugin switches EFS filesystem throughput mode to elastic
type EFSEmergencyPlugin struct {
	fileSystemId      string
	metricLabelName   string
	region            string
	client            *efs.Client
	prometheusAPI     v1.API
	prometheusEnabled bool
}

// Execute implements the ActionPlugin interface
func (p *EFSEmergencyPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
	// Determine the filesystem ID to use
	fileSystemId := p.fileSystemId

	// If Prometheus is enabled and metric label name is configured, query for the label value
	if p.prometheusEnabled && p.metricLabelName != "" {
		labelValue, err := p.queryMetricLabel(ctx, metricName)
		if err != nil {
			log.Warn().
				Err(err).
				Str("metric_name", metricName).
				Str("label_name", p.metricLabelName).
				Msg("failed to query metric label, falling back to EFS_FILE_SYSTEM_ID")
		} else if labelValue != "" {
			fileSystemId = labelValue
			log.Info().
				Str("metric_name", metricName).
				Str("label_name", p.metricLabelName).
				Str("label_value", labelValue).
				Msg("using filesystem ID from metric label")
		} else {
			log.Warn().
				Str("metric_name", metricName).
				Str("label_name", p.metricLabelName).
				Msg("metric label not found in query results, falling back to EFS_FILE_SYSTEM_ID")
		}
	}

	// Validate we have a filesystem ID
	if fileSystemId == "" {
		return fmt.Errorf("no filesystem ID available - set EFS_FILE_SYSTEM_ID or configure EFS_FILE_SYSTEM_PROMETHEUS_LABEL with valid metric label")
	}
	if p.client == nil {
		return fmt.Errorf("AWS client not initialized - check AWS credentials and configuration")
	}

	log.Info().
		Str("metric_name", metricName).
		Float64("value", value).
		Str("threshold", threshold).
		Dur("duration", duration).
		Str("file_system_id", fileSystemId).
		Msg("executing EFS emergency mode: switching to elastic throughput")

	// Update the file system to use elastic throughput
	input := &efs.UpdateFileSystemInput{
		FileSystemId:   aws.String(fileSystemId),
		ThroughputMode: types.ThroughputModeElastic,
	}

	output, err := p.client.UpdateFileSystem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update EFS filesystem throughput mode: %v", err)
	}

	log.Info().
		Str("file_system_id", fileSystemId).
		Str("new_throughput_mode", string(output.ThroughputMode)).
		Str("life_cycle_state", string(output.LifeCycleState)).
		Msg("successfully switched EFS filesystem to elastic throughput mode")

	return nil
}

// queryMetricLabel queries Prometheus for the metric and extracts the specified label value
func (p *EFSEmergencyPlugin) queryMetricLabel(ctx context.Context, metricName string) (string, error) {
	if p.prometheusAPI == nil {
		return "", fmt.Errorf("prometheus API not initialized")
	}

	// Query Prometheus for the metric
	result, warnings, err := p.prometheusAPI.Query(ctx, metricName, time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to query prometheus: %v", err)
	}

	if len(warnings) > 0 {
		log.Warn().
			Strs("warnings", warnings).
			Str("metric_name", metricName).
			Msg("prometheus query returned warnings")
	}

	// Extract label from query results
	if result.Type() == model.ValVector {
		vector := result.(model.Vector)
		if len(vector) > 0 {
			// Get the first sample's labels
			sample := vector[0]
			if labelValue, ok := sample.Metric[model.LabelName(p.metricLabelName)]; ok {
				return string(labelValue), nil
			}
		}
	}

	return "", fmt.Errorf("label %s not found in metric %s", p.metricLabelName, metricName)
}

// Name implements the ActionPlugin interface
func (p *EFSEmergencyPlugin) Name() string {
	return "efs_emergency"
}

// ValidateConfig implements the ActionPlugin interface
func (p *EFSEmergencyPlugin) ValidateConfig() error {
	// At least one of filesystem ID or metric label must be configured
	if p.fileSystemId == "" && p.metricLabelName == "" {
		return fmt.Errorf("at least one of EFS_FILE_SYSTEM_ID or EFS_FILE_SYSTEM_PROMETHEUS_LABEL must be configured")
	}
	
	// AWS client must be initialized
	if p.client == nil {
		return fmt.Errorf("AWS client not initialized - check AWS credentials and configuration")
	}
	
	return nil
}

// Plugin is the exported plugin symbol
var Plugin EFSEmergencyPlugin

func init() {
	// Get EFS filesystem ID from environment (optional if using metric label)
	fileSystemId := os.Getenv("EFS_FILE_SYSTEM_ID")

	// Get metric label name from environment (optional)
	metricLabelName := os.Getenv("EFS_FILE_SYSTEM_PROMETHEUS_LABEL")

	// Get Prometheus endpoint from environment
	prometheusEndpoint := os.Getenv("PROMETHEUS_ENDPOINT")
	if prometheusEndpoint == "" {
		prometheusEndpoint = "http://prometheus:9090"
	}

	// Validate configuration
	if fileSystemId == "" && metricLabelName == "" {
		// Don't fail during tests or when the plugin is not being used
		log.Warn().Msg("Neither EFS_FILE_SYSTEM_ID nor EFS_FILE_SYSTEM_PROMETHEUS_LABEL configured - plugin will fail if executed")
		Plugin = EFSEmergencyPlugin{
			fileSystemId:      "",
			metricLabelName:   "",
			region:            "",
			client:            nil,
			prometheusAPI:     nil,
			prometheusEnabled: false,
		}
		return
	}

	// Get AWS region from environment (optional, will use default if not set)
	region := os.Getenv("AWS_REGION")

	// Load AWS configuration
	// This supports multiple authentication methods:
	// 1. IRSA (IAM Roles for Service Accounts) on EKS - automatically detected
	// 2. EC2 instance profile
	// 3. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// 4. Shared credentials file (~/.aws/credentials)
	ctx := context.Background()

	var cfg aws.Config
	var err error

	if region != "" {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region))
	} else {
		cfg, err = config.LoadDefaultConfig(ctx)
	}

	if err != nil {
		log.Error().Err(err).Msg("failed to load AWS configuration - plugin will fail if executed")
		Plugin = EFSEmergencyPlugin{
			fileSystemId:      fileSystemId,
			metricLabelName:   metricLabelName,
			region:            region,
			client:            nil,
			prometheusAPI:     nil,
			prometheusEnabled: false,
		}
		return
	}

	// Create EFS client
	efsClient := efs.NewFromConfig(cfg)

	// Setup Prometheus client if metric label is configured
	var prometheusAPI v1.API
	prometheusEnabled := false
	if metricLabelName != "" {
		promClient, err := api.NewClient(api.Config{
			Address: prometheusEndpoint,
		})
		if err != nil {
			log.Error().
				Err(err).
				Str("prometheus_endpoint", prometheusEndpoint).
				Msg("failed to create Prometheus client - will use EFS_FILE_SYSTEM_ID if set")
		} else {
			prometheusAPI = v1.NewAPI(promClient)
			prometheusEnabled = true
		}
	}

	Plugin = EFSEmergencyPlugin{
		fileSystemId:      fileSystemId,
		metricLabelName:   metricLabelName,
		region:            cfg.Region,
		client:            efsClient,
		prometheusAPI:     prometheusAPI,
		prometheusEnabled: prometheusEnabled,
	}

	logEvent := log.Info().
		Str("region", cfg.Region).
		Str("prometheus_endpoint", prometheusEndpoint)

	if fileSystemId != "" {
		logEvent = logEvent.Str("file_system_id", fileSystemId)
	}
	if metricLabelName != "" {
		logEvent = logEvent.Str("metric_label", metricLabelName)
	}

	logEvent.Msg("EFS emergency plugin initialized")
}
