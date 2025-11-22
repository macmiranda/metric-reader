package main

import (
	"os"
	"testing"
)

func TestLockNamespaceConfig(t *testing.T) {
	// Save original env vars
	originalLockNamespace := os.Getenv("LOCK_NAMESPACE")
	defer func() {
		if originalLockNamespace != "" {
			os.Setenv("LOCK_NAMESPACE", originalLockNamespace)
		} else {
			os.Unsetenv("LOCK_NAMESPACE")
		}
	}()

	// Test 1: Default config (no env vars)
	os.Unsetenv("LOCK_NAMESPACE")
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Test 1 failed: %v", err)
	}
	if config.LockNamespace != "" {
		t.Errorf("Test 1 failed: expected empty LockNamespace, got %q", config.LockNamespace)
	}

	// Test 2: Set via environment variable
	os.Setenv("LOCK_NAMESPACE", "test-namespace")
	config, err = LoadConfig()
	if err != nil {
		t.Fatalf("Test 2 failed: %v", err)
	}
	if config.LockNamespace != "test-namespace" {
		t.Errorf("Test 2 failed: expected 'test-namespace', got %q", config.LockNamespace)
	}
}
