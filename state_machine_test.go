package main

import (
	"context"
	"testing"
	"time"
)

// Mock plugin for testing
type testPlugin struct {
	name          string
	executeCount  int
	lastValue     float64
	lastThreshold string
	lastDuration  time.Duration
}

func (p *testPlugin) Execute(ctx context.Context, metricName string, value float64, threshold string, duration time.Duration) error {
	p.executeCount++
	p.lastValue = value
	p.lastThreshold = threshold
	p.lastDuration = duration
	return nil
}

func (p *testPlugin) Name() string {
	return p.name
}

func (p *testPlugin) ValidateConfig() error {
	return nil
}

// TestStateTransition_NotBreached_To_SoftActive tests the transition from NotBreached to SoftThresholdActive
func TestStateTransition_NotBreached_To_SoftActive(t *testing.T) {
	// Set leader active for tests
	leaderActive.Store(true)
	defer leaderActive.Store(false)
	
	softPlugin := &testPlugin{name: "soft_plugin"}
	
	state := &stateData{
		currentState: stateNotBreached,
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
	}
	
	// First call: value exceeds threshold but duration not yet met
	processThresholdStateMachine(state, thresholdCfg, 90.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateNotBreached {
		t.Errorf("Expected state to remain NotBreached, got %s", state.currentState)
	}
	
	if state.softThresholdStartTime.IsZero() {
		t.Error("Expected softThresholdStartTime to be set")
	}
	
	if softPlugin.executeCount != 0 {
		t.Errorf("Expected plugin not to be executed yet, but it was called %d times", softPlugin.executeCount)
	}
	
	// Wait and call again to exceed duration
	time.Sleep(100 * time.Millisecond)
	state.softThresholdStartTime = time.Now().Add(-6 * time.Second) // Simulate 6 seconds passed
	
	processThresholdStateMachine(state, thresholdCfg, 90.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateSoftThresholdActive {
		t.Errorf("Expected state to transition to SoftThresholdActive, got %s", state.currentState)
	}
	
	if softPlugin.executeCount != 1 {
		t.Errorf("Expected plugin to be executed once, but it was called %d times", softPlugin.executeCount)
	}
	
	if softPlugin.lastValue != 90.0 {
		t.Errorf("Expected plugin to receive value 90.0, got %f", softPlugin.lastValue)
	}
}

// TestStateTransition_SoftActive_To_NotBreached tests the transition from SoftThresholdActive back to NotBreached
func TestStateTransition_SoftActive_To_NotBreached(t *testing.T) {
	softPlugin := &testPlugin{name: "soft_plugin"}
	
	state := &stateData{
		currentState:           stateSoftThresholdActive,
		softThresholdStartTime: time.Now().Add(-10 * time.Second),
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
	}
	
	// Value no longer exceeds threshold
	processThresholdStateMachine(state, thresholdCfg, 70.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateNotBreached {
		t.Errorf("Expected state to transition to NotBreached, got %s", state.currentState)
	}
	
	if !state.softThresholdStartTime.IsZero() {
		t.Error("Expected softThresholdStartTime to be reset")
	}
}

// TestStateTransition_SoftActive_To_HardActive tests the transition from SoftThresholdActive to HardThresholdActive
func TestStateTransition_SoftActive_To_HardActive(t *testing.T) {
	// Set leader active for tests
	leaderActive.Store(true)
	defer leaderActive.Store(false)
	
	softPlugin := &testPlugin{name: "soft_plugin"}
	hardPlugin := &testPlugin{name: "hard_plugin"}
	
	state := &stateData{
		currentState:           stateSoftThresholdActive,
		softThresholdStartTime: time.Now().Add(-10 * time.Second),
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
		hardThreshold: &threshold{
			value:  100.0,
			plugin: hardPlugin,
		},
	}
	
	// First call: value exceeds hard threshold but duration not yet met
	processThresholdStateMachine(state, thresholdCfg, 110.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateSoftThresholdActive {
		t.Errorf("Expected state to remain SoftThresholdActive, got %s", state.currentState)
	}
	
	if state.hardThresholdStartTime.IsZero() {
		t.Error("Expected hardThresholdStartTime to be set")
	}
	
	if hardPlugin.executeCount != 0 {
		t.Errorf("Expected hard plugin not to be executed yet, but it was called %d times", hardPlugin.executeCount)
	}
	
	// Wait and call again to exceed duration
	state.hardThresholdStartTime = time.Now().Add(-6 * time.Second) // Simulate 6 seconds passed
	
	processThresholdStateMachine(state, thresholdCfg, 110.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateHardThresholdActive {
		t.Errorf("Expected state to transition to HardThresholdActive, got %s", state.currentState)
	}
	
	if hardPlugin.executeCount != 1 {
		t.Errorf("Expected hard plugin to be executed once, but it was called %d times", hardPlugin.executeCount)
	}
	
	if hardPlugin.lastValue != 110.0 {
		t.Errorf("Expected hard plugin to receive value 110.0, got %f", hardPlugin.lastValue)
	}
}

// TestStateTransition_HardActive_To_NotBreached tests the transition from HardThresholdActive back to NotBreached
func TestStateTransition_HardActive_To_NotBreached(t *testing.T) {
	softPlugin := &testPlugin{name: "soft_plugin"}
	hardPlugin := &testPlugin{name: "hard_plugin"}
	
	state := &stateData{
		currentState:           stateHardThresholdActive,
		softThresholdStartTime: time.Now().Add(-20 * time.Second),
		hardThresholdStartTime: time.Now().Add(-10 * time.Second),
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
		hardThreshold: &threshold{
			value:  100.0,
			plugin: hardPlugin,
		},
	}
	
	// Value no longer exceeds either threshold
	processThresholdStateMachine(state, thresholdCfg, 70.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateNotBreached {
		t.Errorf("Expected state to transition to NotBreached, got %s", state.currentState)
	}
	
	if !state.softThresholdStartTime.IsZero() {
		t.Error("Expected softThresholdStartTime to be reset")
	}
	
	if !state.hardThresholdStartTime.IsZero() {
		t.Error("Expected hardThresholdStartTime to be reset")
	}
}

// TestBackoffPeriod_SoftThreshold tests that backoff period prevents re-execution
func TestBackoffPeriod_SoftThreshold(t *testing.T) {
	softPlugin := &testPlugin{name: "soft_plugin"}
	
	state := &stateData{
		currentState:     stateNotBreached,
		softBackoffUntil: time.Now().Add(10 * time.Second), // In backoff for 10 seconds
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
	}
	
	// Try to trigger threshold during backoff
	processThresholdStateMachine(state, thresholdCfg, 90.0, 0, 0, 0, 0, "test_metric", "test_query")
	
	if state.currentState != stateNotBreached {
		t.Errorf("Expected state to remain NotBreached during backoff, got %s", state.currentState)
	}
	
	if softPlugin.executeCount != 0 {
		t.Errorf("Expected plugin not to be executed during backoff, but it was called %d times", softPlugin.executeCount)
	}
}

// TestBackoffPeriod_Expiry tests that plugin re-executes after backoff expires
func TestBackoffPeriod_Expiry(t *testing.T) {
	// Set leader active for tests
	leaderActive.Store(true)
	defer leaderActive.Store(false)
	
	softPlugin := &testPlugin{name: "soft_plugin"}
	
	state := &stateData{
		currentState:           stateSoftThresholdActive,
		softThresholdStartTime: time.Now().Add(-10 * time.Second),
		softBackoffUntil:       time.Now().Add(-1 * time.Second), // Backoff expired
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
	}
	
	// Trigger with value still exceeding threshold after backoff expires
	processThresholdStateMachine(state, thresholdCfg, 90.0, 5*time.Second, 10*time.Second, 5*time.Second, 10*time.Second, "test_metric", "test_query")
	
	if state.currentState != stateSoftThresholdActive {
		t.Errorf("Expected state to remain SoftThresholdActive, got %s", state.currentState)
	}
	
	if softPlugin.executeCount != 1 {
		t.Errorf("Expected plugin to be re-executed after backoff, but it was called %d times", softPlugin.executeCount)
	}
}

// TestLessThanOperator tests that less_than operator works correctly
func TestLessThanOperator(t *testing.T) {
	// Set leader active for tests
	leaderActive.Store(true)
	defer leaderActive.Store(false)
	
	softPlugin := &testPlugin{name: "soft_plugin"}
	
	state := &stateData{
		currentState: stateNotBreached,
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorLessThan,
		softThreshold: &threshold{
			value:  20.0,
			plugin: softPlugin,
		},
	}
	
	// Value below threshold should trigger
	state.softThresholdStartTime = time.Now().Add(-6 * time.Second) // Simulate time passed
	processThresholdStateMachine(state, thresholdCfg, 10.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateSoftThresholdActive {
		t.Errorf("Expected state to transition to SoftThresholdActive with less_than operator, got %s", state.currentState)
	}
	
	if softPlugin.executeCount != 1 {
		t.Errorf("Expected plugin to be executed once, but it was called %d times", softPlugin.executeCount)
	}
}

// TestHardThresholdOnly tests behavior when only hard threshold is configured
func TestHardThresholdOnly(t *testing.T) {
	hardPlugin := &testPlugin{name: "hard_plugin"}
	
	state := &stateData{
		currentState: stateNotBreached,
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		hardThreshold: &threshold{
			value:  100.0,
			plugin: hardPlugin,
		},
	}
	
	// With only hard threshold configured, system should stay in NotBreached
	// According to the state machine, we need to be in SoftThresholdActive to transition to HardThresholdActive
	// Without soft threshold, we can never enter SoftThresholdActive, so hard threshold is unreachable
	processThresholdStateMachine(state, thresholdCfg, 110.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	// State should remain NotBreached since we can't go directly to HardThresholdActive
	if state.currentState != stateNotBreached {
		t.Errorf("Expected state to remain NotBreached when only hard threshold configured, got %s", state.currentState)
	}
}

// TestSoftThresholdOnly tests behavior when only soft threshold is configured
func TestSoftThresholdOnly(t *testing.T) {
	// Set leader active for tests
	leaderActive.Store(true)
	defer leaderActive.Store(false)
	
	softPlugin := &testPlugin{name: "soft_plugin"}
	
	state := &stateData{
		currentState: stateNotBreached,
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
	}
	
	// Should transition to SoftThresholdActive
	state.softThresholdStartTime = time.Now().Add(-6 * time.Second)
	processThresholdStateMachine(state, thresholdCfg, 90.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateSoftThresholdActive {
		t.Errorf("Expected state to transition to SoftThresholdActive, got %s", state.currentState)
	}
	
	if softPlugin.executeCount != 1 {
		t.Errorf("Expected soft plugin to be executed once, but it was called %d times", softPlugin.executeCount)
	}
}

// TestNonLeaderDoesNotExecutePlugin tests that plugins are not executed when not leader
func TestNonLeaderDoesNotExecutePlugin(t *testing.T) {
	// Ensure we're not leader
	leaderActive.Store(false)
	defer leaderActive.Store(false)
	
	softPlugin := &testPlugin{name: "soft_plugin"}
	
	state := &stateData{
		currentState: stateNotBreached,
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
	}
	
	// Simulate threshold already exceeded for duration
	state.softThresholdStartTime = time.Now().Add(-6 * time.Second)
	processThresholdStateMachine(state, thresholdCfg, 90.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	// State should transition even if not leader
	if state.currentState != stateSoftThresholdActive {
		t.Errorf("Expected state to transition to SoftThresholdActive even when not leader, got %s", state.currentState)
	}
	
	// Plugin should NOT be executed when not leader
	if softPlugin.executeCount != 0 {
		t.Errorf("Expected plugin NOT to be executed when not leader, but it was called %d times", softPlugin.executeCount)
	}
}

// TestOnlyRelevantThresholdsChecked verifies optimization that only relevant thresholds are checked per state
func TestOnlyRelevantThresholdsChecked(t *testing.T) {
	// This is more of a code review test - we verify the behavior works correctly
	// The actual optimization is in the implementation where thresholds are only checked when needed
	
	// Set leader active for tests
	leaderActive.Store(true)
	defer leaderActive.Store(false)
	
	softPlugin := &testPlugin{name: "soft_plugin"}
	hardPlugin := &testPlugin{name: "hard_plugin"}
	
	state := &stateData{
		currentState: stateNotBreached,
	}
	
	thresholdCfg := &thresholdConfig{
		operator: thresholdOperatorGreaterThan,
		softThreshold: &threshold{
			value:  80.0,
			plugin: softPlugin,
		},
		hardThreshold: &threshold{
			value:  100.0,
			plugin: hardPlugin,
		},
	}
	
	// In NotBreached state with value only exceeding soft threshold
	// Only soft threshold should be processed
	state.softThresholdStartTime = time.Now().Add(-6 * time.Second)
	processThresholdStateMachine(state, thresholdCfg, 90.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateSoftThresholdActive {
		t.Errorf("Expected transition to SoftThresholdActive, got %s", state.currentState)
	}
	
	if softPlugin.executeCount != 1 {
		t.Errorf("Expected soft plugin to execute once, got %d", softPlugin.executeCount)
	}
	
	if hardPlugin.executeCount != 0 {
		t.Errorf("Expected hard plugin NOT to execute in NotBreached state, got %d executions", hardPlugin.executeCount)
	}
	
	// Now in SoftThresholdActive, exceed hard threshold
	state.hardThresholdStartTime = time.Now().Add(-6 * time.Second)
	processThresholdStateMachine(state, thresholdCfg, 110.0, 5*time.Second, 0, 5*time.Second, 0, "test_metric", "test_query")
	
	if state.currentState != stateHardThresholdActive {
		t.Errorf("Expected transition to HardThresholdActive, got %s", state.currentState)
	}
	
	if hardPlugin.executeCount != 1 {
		t.Errorf("Expected hard plugin to execute once, got %d", hardPlugin.executeCount)
	}
}
