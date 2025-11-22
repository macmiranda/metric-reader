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
	"github.com/rs/zerolog/log"
)

// EFSEmergencyPlugin switches EFS filesystem throughput mode to elastic
type EFSEmergencyPlugin struct {
	fileSystemId string
	region       string
	client       *efs.Client
}

// Execute implements the ActionPlugin interface
func (p *EFSEmergencyPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
	log.Info().
		Str("metric_name", metricName).
		Float64("value", value).
		Str("threshold", threshold).
		Dur("duration", duration).
		Str("file_system_id", p.fileSystemId).
		Msg("executing EFS emergency mode: switching to elastic throughput")

	// Update the file system to use elastic throughput
	input := &efs.UpdateFileSystemInput{
		FileSystemId:   aws.String(p.fileSystemId),
		ThroughputMode: types.ThroughputModeElastic,
	}

	output, err := p.client.UpdateFileSystem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update EFS filesystem throughput mode: %v", err)
	}

	log.Info().
		Str("file_system_id", p.fileSystemId).
		Str("new_throughput_mode", string(output.ThroughputMode)).
		Str("life_cycle_state", string(output.LifeCycleState)).
		Msg("successfully switched EFS filesystem to elastic throughput mode")

	return nil
}

// Name implements the ActionPlugin interface
func (p *EFSEmergencyPlugin) Name() string {
	return "efs_emergency"
}

// Plugin is the exported plugin symbol
var Plugin EFSEmergencyPlugin

func init() {
	// Get EFS filesystem ID from environment
	fileSystemId := os.Getenv("EFS_FILE_SYSTEM_ID")
	if fileSystemId == "" {
		log.Fatal().Msg("EFS_FILE_SYSTEM_ID environment variable is required for efs_emergency plugin")
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
		log.Fatal().Err(err).Msg("failed to load AWS configuration")
	}

	// Create EFS client
	client := efs.NewFromConfig(cfg)

	Plugin = EFSEmergencyPlugin{
		fileSystemId: fileSystemId,
		region:       cfg.Region,
		client:       client,
	}

	log.Info().
		Str("file_system_id", fileSystemId).
		Str("region", cfg.Region).
		Msg("EFS emergency plugin initialized")
}
