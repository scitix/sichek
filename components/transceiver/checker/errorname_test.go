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
package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/require"
)

// TestCheckers_PopulateErrorNameOnHealthyPath guards against the stuck-metric
// field bug: the Prometheus exporter keys reset-then-set on Item+"_"+ErrorName,
// so a healthy CheckerResult that leaves ErrorName empty can never clear a
// previously-set abnormal series (it stays at 1 until the daemon restarts).
// Every transceiver checker must therefore carry its ErrorName on the healthy
// path too, not only when it flags a fault.
func TestCheckers_PopulateErrorNameOnHealthyPath(t *testing.T) {
	checkers, err := NewCheckers(nil, testSpec())
	require.NoError(t, err)
	require.NotEmpty(t, checkers)

	// With no modules present, every checker returns its initial healthy result.
	info := &collector.TransceiverInfo{Modules: nil}
	for _, chk := range checkers {
		t.Run(chk.Name(), func(t *testing.T) {
			res, err := chk.Check(context.Background(), info)
			require.NoError(t, err)
			require.Equal(t, consts.StatusNormal, res.Status)
			require.NotEmpty(t, res.ErrorName,
				"healthy CheckerResult must carry ErrorName so its metric can self-clear on recovery")
		})
	}
}
