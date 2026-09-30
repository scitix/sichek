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

package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/checker"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/consts"
)

// TestExportMetrics_SignalIntegrityClearsAfterRecovery reproduces the field bug
// where sichek_transceiver_badsignalintegrity stayed stuck at 1 forever after a
// single transient "Bad signal integrity" reading, even once mlxlink reported
// "No issue was observed" again (replacing the optical module never cleared it).
//
// The exporter does reset-then-set keyed on Item+"_"+ErrorName. If a checker
// leaves ErrorName empty on its healthy path, the reset targets the wrong
// (empty) metric name and the abnormal series is never cleared. This test drives
// the REAL SignalIntegrityChecker output through the exporter across an
// abnormal→normal transition and asserts the series is gone after recovery.
func TestExportMetrics_SignalIntegrityClearsAfterRecovery(t *testing.T) {
	m := newHealthCheckResMetrics()
	chk := &checker.SignalIntegrityChecker{}
	const item = "transceiver"

	check := func(rec string) *common.Result {
		info := &collector.TransceiverInfo{Modules: []collector.ModuleInfo{
			{Interface: "ib1", NetworkType: "business", Present: true, Recommendation: rec},
		}}
		res, err := chk.Check(context.Background(), info)
		require.NoError(t, err)
		return &common.Result{Item: item, Checkers: []*common.CheckerResult{res}}
	}

	// Cycle 1: a transient bad reading sets the gauge.
	bad := check("Bad signal integrity")
	require.Equal(t, consts.StatusAbnormal, bad.Checkers[0].Status)
	m.ExportMetrics(bad)
	require.Equal(t, 1, countSeries(t, m, item+"_BadSignalIntegrity"),
		"abnormal reading should set exactly one series")

	// Cycle 2: hardware recovered, mlxlink now clean.
	good := check("No issue was observed")
	require.Equal(t, consts.StatusNormal, good.Checkers[0].Status)
	m.ExportMetrics(good)
	require.Equal(t, 0, countSeries(t, m, item+"_BadSignalIntegrity"),
		"series must clear once the checker reports healthy again")
}
