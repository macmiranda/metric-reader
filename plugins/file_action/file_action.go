package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// FileActionPlugin creates a file with configurable size
type FileActionPlugin struct {
	outputDir string
	fileSize  int64
}

// Execute implements the ActionPlugin interface
func (p *FileActionPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
	// Create filename with timestamp
	filename := fmt.Sprintf("metric_%s_%d.bin", metricName, time.Now().Unix())
	filepath := filepath.Join(p.outputDir, filename)

	// Create file
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Write data
	if err := file.Truncate(p.fileSize); err != nil {
		return fmt.Errorf("failed to set file size: %v", err)
	}

	// Flush to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %v", err)
	}

	log.Info().
		Str("file", filepath).
		Int64("size", p.fileSize).
		Msg("created file")

	return nil
}

// Name implements the ActionPlugin interface
func (p *FileActionPlugin) Name() string {
	return "file_action"
}

// ValidateConfig implements the ActionPlugin interface
func (p *FileActionPlugin) ValidateConfig() error {
	// Output directory should always be set (either from config or default)
	// but validate it's not empty as a sanity check
	if p.outputDir == "" {
		return fmt.Errorf("FILE_ACTION_DIR is empty - plugin not properly initialized")
	}
	
	// Check that file size is valid
	if p.fileSize <= 0 {
		return fmt.Errorf("FILE_ACTION_SIZE must be greater than 0, got %d", p.fileSize)
	}
	
	// Verify the directory exists and is writable
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return fmt.Errorf("cannot create or access FILE_ACTION_DIR '%s': %v", p.outputDir, err)
	}
	
	return nil
}

// Plugin is the exported plugin symbol
var Plugin FileActionPlugin

func init() {
	// Get output directory from environment
	outputDir := os.Getenv("FILE_ACTION_DIR")
	if outputDir == "" {
		outputDir = "/tmp/metric-files"
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal().Err(err).Str("dir", outputDir).Msg("failed to create output directory")
	}

	// Get file size from environment (default to 1MB)
	fileSize := int64(1024 * 1024) // 1MB default
	if sizeStr := os.Getenv("FILE_ACTION_SIZE"); sizeStr != "" {
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			log.Fatal().Err(err).Str("size", sizeStr).Msg("invalid FILE_ACTION_SIZE value")
		}
		fileSize = size
	}

	Plugin = FileActionPlugin{
		outputDir: outputDir,
		fileSize:  fileSize,
	}
}
