package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	filter "github.com/scitix/sichek/components/common/eventfilter"
)

func TestCollector_Collect(t *testing.T) {
	// Mock context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock configuration
	cfg := &CpuEventRules{
		Rules: common.EventRuleGroup{ // Updated field name to EventRules
			"testChecker": &common.EventRuleConfig{
				Name:        "testChecker",
				Description: "Test checker",
				LogFile:     "/var/log/test.log",
				Regexp:      "test",
				Level:       "warning",
				Suggestion:  "test suggestion",
			},
		},
	}
	// Create a temporary log file for testing in a goroutine
	var logFile *os.File
	var err error
	logFile, err = os.CreateTemp("", "test.log")
	if err != nil {
		t.Fatalf("Failed to create temp log file: %v", err)
	}
	t.Logf("Log file: %s", logFile.Name())
	cfg.Rules["testChecker"].LogFile = logFile.Name()
	// Write some test data to the log file
	_, err = logFile.WriteString("test log data\n")
	if err != nil {
		t.Fatalf("Failed to write to temp log file: %+v", err)
	}
	err = logFile.Close()
	if err != nil {
		t.Errorf("Failed to close temp log file: %v", err)
	}

	// Start a goroutine to write to the log file continuously
	go func() {
		for {
			select {
			case <-ctx.Done():
				err := os.Remove(logFile.Name())
				if err != nil {
					t.Errorf("Failed to remove temp log file: %v", err)
				}
				return
			default:
				file, err := os.OpenFile(logFile.Name(), os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Errorf("Failed to open log file: %v", err)
					return
				}
				_, err = file.WriteString("additional test log data\n")
				if err != nil {
					t.Errorf("Failed to write to log file: %v", err)
					err := file.Close()
					if err != nil {
						t.Errorf("Failed to close temp log file: %v", err)
					}
					return
				}
				err = file.Close()
				if err != nil {
					t.Errorf("Failed to close temp log file: %v", err)
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Update the log file path in the configuration
	cfg.Rules["testChecker"].LogFile = logFile.Name()
	t.Logf("Event checkers: %+v", cfg.Rules["testChecker"])
	// Create a new EventFilter instance
	filterPointer, err := filter.NewEventFilter("testChecker", cfg.Rules, 1, 100)
	if err != nil {
		t.Fatalf("Failed to create file filter: %v", err)
	}

	// Read the current content of the log file
	content, err := os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file0: %v", err)
	}
	t.Logf("Current log file content: %s", string(content))

	result := filterPointer.Check()
	time.Sleep(2 * time.Second)
	// Read the current content of the log file
	content, err = os.ReadFile(logFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file1: %v", err)
	}
	t.Logf("Current log file content: %s", string(content))
	// Call the Collect method
	result = filterPointer.Check()

	// Verify the results
	if len(result.Checkers) == 0 {
		t.Errorf("Expected non-empty event results, got empty")
	}
}
