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
	"testing"
	"time"

	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func panickyIndex(s []string) string {
	return s[0]
}

func TestRunHealthCheckWithTimeout_PanicReportsSite(t *testing.T) {
	res, err := RunHealthCheckWithTimeout(context.Background(), time.Second, "fake",
		func(ctx context.Context) (*Result, error) {
			panickyIndex(nil)
			return nil, nil
		})
	require.NoError(t, err)
	require.NotNil(t, res)
	var c *CheckerResult
	for _, cr := range res.Checkers {
		if cr.Name == "fakeHealthCheckPanic" {
			c = cr
		}
	}
	require.NotNil(t, c, "panic checker result missing")
	assert.Equal(t, "fakeHealthCheckPanic", c.ErrorName)
	assert.Equal(t, consts.LevelCritical, c.Level)
	assert.Contains(t, c.Description, "index out of range [0] with length 0")
	assert.Contains(t, c.Detail, "common.panickyIndex")
	assert.Contains(t, c.Detail, "helper_test.go:")
}
