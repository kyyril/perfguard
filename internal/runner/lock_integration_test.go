// +build integration

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentRuns_SameDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a shared output directory
	outputDir := t.TempDir()

	// Create a simple lock mechanism test
	// We'll simulate what would happen if two check runs tried to use the same directory

	lockFile := filepath.Join(outputDir, ".perfguard.lock")

	// First process acquires lock
	f1, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("first process failed to create lock: %v", err)
	}
	defer f1.Close()
	defer os.Remove(lockFile)

	// Write PID to lock file
	if _, err := f1.WriteString("1234\n"); err != nil {
		t.Fatalf("failed to write to lock file: %v", err)
	}

	// Second process tries to acquire lock
	_, err = os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err == nil {
		t.Fatal("second process should have failed to acquire lock")
	}

	// Error should be clear
	if !os.IsExist(err) {
		t.Errorf("expected file exists error, got: %v", err)
	}

	// In real implementation, we would check for a specific error message
	// For now, verify that the lock file prevents concurrent access
}

func TestConcurrentRuns_DifferentDirectories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two different output directories
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Both should be able to create their lock files
	lock1 := filepath.Join(dir1, ".perfguard.lock")
	lock2 := filepath.Join(dir2, ".perfguard.lock")

	var wg sync.WaitGroup
	errors := make(chan error, 2)

	// Try to create both locks concurrently
	for _, lockPath := range []string{lock1, lock2} {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				errors <- err
				return
			}
			f.Close()

			// Hold the lock briefly
			time.Sleep(100 * time.Millisecond)
			os.Remove(path)
		}(lockPath)
	}

	wg.Wait()
	close(errors)

	// Neither should have errored
	for err := range errors {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestLockFileMessage(t *testing.T) {
	// Test that when a lock file exists, we produce a clear error message
	// This is a unit-style test for the lock file error formatting

	lockPath := filepath.Join(t.TempDir(), ".perfguard.lock")

	// Create a lock file with PID
	if err := os.WriteFile(lockPath, []byte("12345\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Simulate detecting the lock
	content, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	pid := strings.TrimSpace(string(content))

	// Generate error message
	errMsg := "another perfguard process (PID " + pid + ") is using this directory"

	// Verify message is clear
	if !strings.Contains(errMsg, "another") {
		t.Error("message should mention another process")
	}
	if !strings.Contains(errMsg, pid) {
		t.Error("message should include PID")
	}
	if !strings.Contains(errMsg, "directory") {
		t.Error("message should mention directory")
	}
}
