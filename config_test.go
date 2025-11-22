package main

import (
	"os"
	"testing"
)

func TestLockNamespaceConfig(t *testing.T) {
	// Save original env vars
	originalLockNamespace := os.Getenv("LEADER_ELECTION_LOCK_NAMESPACE")
	defer func() {
		if originalLockNamespace != "" {
			os.Setenv("LEADER_ELECTION_LOCK_NAMESPACE", originalLockNamespace)
		} else {
			os.Unsetenv("LEADER_ELECTION_LOCK_NAMESPACE")
		}
	}()

	// Test 1: Default config (no env vars)
	os.Unsetenv("LEADER_ELECTION_LOCK_NAMESPACE")
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Test 1 failed: %v", err)
	}
	if config.LeaderElectionLockNamespace != "" {
		t.Errorf("Test 1 failed: expected empty LeaderElectionLockNamespace, got %q", config.LeaderElectionLockNamespace)
	}

	// Test 2: Set via environment variable
	os.Setenv("LEADER_ELECTION_LOCK_NAMESPACE", "test-namespace")
	config, err = LoadConfig()
	if err != nil {
		t.Fatalf("Test 2 failed: %v", err)
	}
	if config.LeaderElectionLockNamespace != "test-namespace" {
		t.Errorf("Test 2 failed: expected 'test-namespace', got %q", config.LeaderElectionLockNamespace)
	}
}
