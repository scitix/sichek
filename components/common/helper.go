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
	"time"

	"github.com/scitix/sichek/consts"
)

// RunHealthCheckWithContext wraps the HealthCheck call and ensures it respects the provided context timeout or cancellation
func RunHealthCheckWithTimeout(ctx context.Context, timeout time.Duration, componentName string, fn func(ctx context.Context) (*Result, error)) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout) // Use the timeout context
	defer cancel()
	// Create channels for result and error
	resultChan := make(chan *Result) // Channel for result
	errorChan := make(chan error)    // Channel for error

	// Run the function in a goroutine
	go func() {
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
		Level:    consts.LevelCritical,
		Checkers: []*CheckerResult{timeoutCheckerResult},
		Time:     time.Now(),
	}

	return timeoutResult, nil
}

// handleResult processes the result of the health check
func handleResult(result *Result, componentName string) (*Result, error) {
	timeoutResolvedResult := &CheckerResult{
		Name:        fmt.Sprintf("%sHealthCheckTimeout", componentName),
		Description: fmt.Sprintf("component %s health check resolved", componentName),
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		ErrorName:   fmt.Sprintf("%sHealthCheckTimeout", componentName),
	}

	result.Checkers = append(result.Checkers, timeoutResolvedResult)
	return result, nil
}
