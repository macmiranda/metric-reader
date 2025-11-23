# Agent Instructions for metric-reader

This document provides guidance for AI agents working with the metric-reader codebase.

## Configuration Structure

The metric-reader now supports two configuration structures for threshold settings:

### New Recommended Structure (v2 - Nested Sections)

Use separate `[soft]` and `[hard]` sections in `config.toml`:

```toml
threshold_operator = "greater_than"

[soft]
threshold = 80.0
plugin = "log_action"
duration = "30s"
backoff_delay = "1m"

[hard]
threshold = 100.0
plugin = "file_action"
duration = "30s"
backoff_delay = "1m"
```

Each section supports:
- `threshold` (float64): The threshold value
- `plugin` (string): Plugin name to execute when threshold is exceeded
- `duration` (duration): How long the threshold must be exceeded before triggering
- `backoff_delay` (duration): Delay between repeated plugin executions

### Legacy Structure (v1 - Flat Configuration)

The old flat structure is still supported for backward compatibility:

```toml
threshold_operator = "greater_than"
soft_threshold = 80.0
hard_threshold = 100.0
soft_threshold_plugin = "log_action"
hard_threshold_plugin = "file_action"
threshold_duration = "30s"  # Shared across both thresholds
backoff_delay = "1m"        # Shared across both thresholds
```

### Environment Variables

Both structures can be configured via environment variables:

**New structure:**
- `SOFT_THRESHOLD`, `SOFT_DURATION`, `SOFT_BACKOFF_DELAY`
- `HARD_THRESHOLD`, `HARD_DURATION`, `HARD_BACKOFF_DELAY`

**Legacy structure (still supported):**
- `SOFT_THRESHOLD`, `HARD_THRESHOLD`
- `SOFT_THRESHOLD_PLUGIN`, `HARD_THRESHOLD_PLUGIN`
- `THRESHOLD_DURATION` (shared)
- `BACKOFF_DELAY` (shared)

## Backward Compatibility

The configuration system automatically handles migration between the two structures:

1. If new `[soft]`/`[hard]` sections are found, they take precedence
2. If only legacy flat fields are found, they are migrated to the new structure internally
3. Both structures are kept in sync for backward compatibility in the code

## Testing Configuration Changes

When modifying configuration handling:

1. Test the new nested structure with `TestNestedThresholdConfig`
2. Test legacy flat structure with `TestLegacyFlatThresholdConfig`
3. Test environment variables with `TestEnvironmentVariableThresholdConfig`
4. Ensure backward compatibility by running all existing tests

## Code Organization

### Key Files

- `config.go`: Configuration loading and structure definitions
  - `ThresholdSection`: Struct for soft/hard threshold configuration
  - `Config`: Main configuration struct
  - `LoadConfig()`: Loads and migrates configuration

- `main.go`: Application logic
  - `processThresholdStateMachine()`: Handles threshold state transitions
  - Accepts separate duration/backoff_delay for soft and hard thresholds

- `config_test.go`: Configuration tests
  - Tests for new nested structure
  - Tests for legacy flat structure
  - Tests for environment variables

### State Machine Updates

The threshold state machine now supports independent timing for soft and hard thresholds:

```go
processThresholdStateMachine(
    state, 
    thresholdCfg, 
    value, 
    softDuration,        // Separate duration for soft threshold
    softBackoffDelay,    // Separate backoff for soft threshold
    hardDuration,        // Separate duration for hard threshold
    hardBackoffDelay,    // Separate backoff for hard threshold
    metricName, 
    query
)
```

## Making Changes

### Adding New Configuration Options

1. Add fields to the appropriate struct in `config.go` (e.g., `ThresholdSection` for threshold-related options)
2. Update `LoadConfig()` to bind environment variables
3. Update backward compatibility logic if needed
4. Add tests in `config_test.go`
5. Update `config.toml.example`
6. Update README.md environment variable table

### Testing Your Changes

```bash
# Run all tests
go test -v ./...

# Run specific test
go test -v -run TestNestedThresholdConfig

# Check test coverage
go test -cover ./...
```

## Best Practices

1. **Always maintain backward compatibility** - Users may have existing configurations
2. **Test both new and legacy structures** - Ensure migrations work correctly
3. **Update documentation** - README.md, config.toml.example, and this file
4. **Use descriptive test names** - Make it clear what configuration structure is being tested
5. **Keep the state machine flexible** - Allow independent configuration of soft and hard thresholds

## Common Pitfalls

1. **Don't break backward compatibility** - Legacy configurations must continue to work
2. **Don't forget to sync structures** - The migration code keeps old and new fields in sync
3. **Don't assume shared settings** - Soft and hard thresholds can have different durations/backoffs
4. **Don't skip tests** - Configuration handling is complex and needs thorough testing

## Questions?

If you have questions about the configuration system or need clarification on any aspect of the codebase, refer to:
- README.md for user-facing documentation
- config.go source code for implementation details
- config_test.go for usage examples
