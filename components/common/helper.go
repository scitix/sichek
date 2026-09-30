/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

// RunHealthCheckWithContext wraps the HealthCheck call and ensures it respects the provided context timeout or cancellation
func RunHealthCheckWithTimeout(ctx context.Context, timeout time.Duration, componentName string, fn func(ctx context.Context) (*Result, error)) (*Result, error) {
	timer := NewTimer(fmt.Sprintf("%s-HealthCheck-Cost", componentName))
	defer timer.Total()
	ctx, cancel := context.WithTimeout(ctx, timeout) // Use the timeout context
	defer cancel()
	// Create channels for result and error
	resultChan := make(chan *Result) // Channel for result
	errorChan := make(chan error)    // Channel for error

	// Run the function in a goroutine; recover from panic so the process does not crash
	// and a panic result can be returned and exported as an anomaly metric.
	// Note: CGO segfault (e.g. in NVML) cannot be recovered and will still kill the process.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithField("component", componentName).Errorf("HealthCheck panic recovered: %v", r)
				resultChan <- createPanicResult(componentName, r)
			}
		}()
		res, err := fn(ctx)
		if err != nil {
			errorChan <- err // Send error to the error channel
		} else {
			resultChan <- res // Send result to the result channel
		}
	}()

	// Wait for either the function to finish or the context to be done
	select {
	case <-ctx.Done():
		return createTimeoutResult(componentName, timeout)
	case err := <-errorChan:
		return nil, err
	case result := <-resultChan:
		return handleResult(result, componentName)
	}
}

// createTimeoutResult returns a timeout error result
func createTimeoutResult(componentName string, timeout time.Duration) (*Result, error) {
	timeoutCheckerResult := &CheckerResult{
		Name:        fmt.Sprintf("%sHealthCheckTimeout", componentName),
		Description: fmt.Sprintf("component %s did not return a result within %v", componentName, timeout),
		Status:      consts.StatusAbnormal,
		Level:       consts.LevelCritical,
		ErrorName:   fmt.Sprintf("%sHealthCheckTimeout", componentName),
		Suggestion:  fmt.Sprintf("Please check the %s status", componentName),
	}

	timeoutResult := &Result{
		Item:     componentName,
		Status:   consts.StatusAbnormal,
		Level:    timeoutCheckerResult.Level,
		Checkers: []*CheckerResult{timeoutCheckerResult},
		Time:     time.Now(),
	}

	return timeoutResult, nil
}

// createPanicResult returns a result for HealthCheck panic/segfault-like failure so that
// the program does not crash and the anomaly can be exported as a metric (e.g. nvidia_nvidiaHealthCheckPanic).
func createPanicResult(componentName string, panicValue interface{}) *Result {
	panicName := fmt.Sprintf("%sHealthCheckPanic", componentName)
	desc := fmt.Sprintf("component %s HealthCheck panicked: %v", componentName, panicValue)
	panicCheckerResult := &CheckerResult{
		Name:        panicName,
		Description: desc,
		Status:      consts.StatusAbnormal,
		Level:       consts.LevelCritical,
		Detail:      fmt.Sprint(panicValue),
		ErrorName:   panicName,
		Suggestion:  fmt.Sprintf("Please check the %s status (e.g. GPU/NVML stability)", componentName),
	}
	return &Result{
		Item:     componentName,
		Status:   consts.StatusAbnormal,
		Level:    panicCheckerResult.Level,
		Checkers: []*CheckerResult{panicCheckerResult},
		Time:     time.Now(),
	}
}

// handleResult processes the result of the health check
func handleResult(result *Result, componentName string) (*Result, error) {
	timeoutResolvedResult := &CheckerResult{
		Name:        fmt.Sprintf("%sHealthCheckTimeout", componentName),
		Description: fmt.Sprintf("component %s health check resolved", componentName),
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		ErrorName:   fmt.Sprintf("%sHealthCheckTimeout", componentName),
	}
	if result == nil {
		logrus.WithField("component", componentName).Warn("health check returned nil result")
		return nil, nil
	}
	result.Checkers = append(result.Checkers, timeoutResolvedResult)
	return result, nil
}

type Timer struct {
	start     time.Time
	stepStart time.Time
	name      string
}

func NewTimer(name string) *Timer {
	return &Timer{
		start:     time.Now(),
		stepStart: time.Now(),
		name:      name,
	}
}

func (t *Timer) Mark(step string) {
	elapsed := time.Since(t.stepStart)
	logrus.WithFields(logrus.Fields{
		"func": t.name,
		"step": step,
		"cost": elapsed,
	}).Info("Step timing")
	t.stepStart = time.Now()
}

func (t *Timer) Total() {
	total := time.Since(t.start)
	logrus.WithFields(logrus.Fields{
		"func":  t.name,
		"total": total,
	}).Info("Total execution time")
}

func ExtractAndDeduplicate(curr string) string {
	// Split the string by ';'
	values := strings.Split(curr, ",")

	// Use a map to store unique values
	uniqueValues := make(map[string]struct{})
	for _, value := range values {
		if value != "" { // Ignore empty strings
			uniqueValues[value] = struct{}{}
		}
	}

	// Collect keys from the map into a slice
	result := make([]string, 0, len(uniqueValues))
	for key := range uniqueValues {
		result = append(result, key)
	}

	// Join the unique values back into a single string
	return strings.Join(result, ",")
}
