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

// RunHealthCheckWithContext wraps HealthCheck call and makes sure it respects the provided context timeout or cancellation
func RunHealthCheckWithContext(ctx context.Context, timeout time.Duration, componentName string, fn func(ctx context.Context) (*Result, error)) (*Result, error) {
	_, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create a channel to receive the result
	done := make(chan struct {
		result *Result
		err    error
	})
	// Run the function in a goroutine
	go func() {
		res, err := fn(ctx)
		done <- struct {
			result *Result
			err    error
		}{result: res, err: err}
	}()
	// Wait for either the function to finish or the context to be done
	select {
	case <-ctx.Done():
		// If the context is done, return a cancellation/timed-out error
		timeoutCheckerResult := &CheckerResult{
			Name:        fmt.Sprintf("%sHealthCheckTimeout", componentName),
			Description: fmt.Sprintf("component %s did not return a result within %v s", componentName, timeout),
			Status:      consts.StatusAbnormal,
			Level:       consts.LevelCritical,
			Detail:      "",
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
		return timeoutResult, fmt.Errorf("%sHealthCheckTimeout: did not return a result within %v s", componentName, timeout)
	case result := <-done:
		return result.result, result.err
	}
}
