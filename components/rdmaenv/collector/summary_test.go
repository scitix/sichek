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

package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSummaryFromSample(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "metrics_sample.txt"))
	require.NoError(t, err)
	sum := BuildSummary(ParseMetrics(string(body), "rdma_env_pre_"))

	assert.Equal(t, "drift", sum.HostCompliance)
	assert.Equal(t, map[string]int{"converged": 2, "drift": 1, "pending_reboot": 1}, sum.VerdictCounts)

	// 4 knob_verdict rows, sorted by (device, knob).
	require.Len(t, sum.Knobs, 4)

	// Numeric knob: desired/observed come from the value families.
	ipgDrift := findKnob(sum.Knobs, "0000:ef:00.3", "qos.ipg")
	require.NotNil(t, ipgDrift)
	assert.Equal(t, "drift", ipgDrift.Verdict)
	assert.Equal(t, "25", ipgDrift.Desired)
	assert.Equal(t, "3", ipgDrift.Observed)

	// pending_reboot knob carries reboot_required=true.
	vfs := findKnob(sum.Knobs, "0000:19:00.0", "firmware.NUM_OF_VFS")
	require.NotNil(t, vfs)
	assert.Equal(t, "pending_reboot", vfs.Verdict)
	assert.True(t, vfs.RebootRequired)
	assert.Equal(t, "16", vfs.Desired)
	assert.Equal(t, "8", vfs.Observed)

	// Non-numeric knob: desired/observed come from knob_info labels.
	trust := findKnob(sum.Knobs, "eth_r0_p0", "qos.trust")
	require.NotNil(t, trust)
	assert.Equal(t, "dscp", trust.Desired)
	assert.Equal(t, "dscp", trust.Observed)

	// Observe-only inventory.
	require.Len(t, sum.Observes, 1)
	assert.Equal(t, "pcie.link_width", sum.Observes[0].Knob)
	assert.Equal(t, "16", sum.Observes[0].Observed)

	// SeriesTotal counts every passed-through series (incl. agent_up, build_info).
	assert.Equal(t, 18, sum.SeriesTotal)
}

func TestBuildSummaryEmpty(t *testing.T) {
	sum := BuildSummary(map[string][]Series{})
	assert.Equal(t, "", sum.HostCompliance)
	assert.Empty(t, sum.Knobs)
	assert.Equal(t, 0, sum.SeriesTotal)
	assert.NotNil(t, sum.VerdictCounts)
}

func findKnob(knobs []KnobView, device, knob string) *KnobView {
	for i := range knobs {
		if knobs[i].Device == device && knobs[i].Knob == knob {
			return &knobs[i]
		}
	}
	return nil
}
