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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/rdmaenv/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleValue(t *testing.T, reg *prometheus.Registry, name string, wantLabels map[string]string) (float64, bool) {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labelsEqual(labels, wantLabels) {
				return m.GetGauge().GetValue(), true
			}
		}
	}
	return 0, false
}

func sampleCount(t *testing.T, reg *prometheus.Registry, name string) int {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			return len(mf.GetMetric())
		}
	}
	return 0
}

func labelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestExportMetricsPassthrough(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newRdmaEnvMetricsWith(reg)

	info := &collector.Info{
		Available: true,
		Series: map[string][]collector.Series{
			"rdma_env_pre_knob_verdict": {
				{Name: "rdma_env_pre_knob_verdict", Labels: map[string]string{"device": "d0", "knob": "k0", "verdict": "drift"}, Value: 1},
				{Name: "rdma_env_pre_knob_verdict", Labels: map[string]string{"device": "d1", "knob": "k1", "verdict": "converged"}, Value: 1},
			},
			"rdma_env_pre_knob_observed_value": {
				{Name: "rdma_env_pre_knob_observed_value", Labels: map[string]string{"device": "d0", "knob": "k0"}, Value: 3},
			},
			"rdma_env_pre_agent_up": {
				{Name: "rdma_env_pre_agent_up", Labels: map[string]string{}, Value: 1},
			},
		},
	}
	m.ExportMetrics(info)

	// Values preserved verbatim, no node label added.
	v, ok := sampleValue(t, reg, "rdma_env_pre_knob_observed_value", map[string]string{"device": "d0", "knob": "k0"})
	require.True(t, ok)
	assert.Equal(t, float64(3), v)
	assert.Equal(t, 2, sampleCount(t, reg, "rdma_env_pre_knob_verdict"))
	assert.Equal(t, 1, sampleCount(t, reg, "rdma_env_pre_agent_up"))
}

func TestExportMetricsResetsStaleSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newRdmaEnvMetricsWith(reg)

	m.ExportMetrics(&collector.Info{
		Available: true,
		Series: map[string][]collector.Series{
			"rdma_env_pre_knob_verdict": {
				{Name: "rdma_env_pre_knob_verdict", Labels: map[string]string{"device": "d0", "knob": "k0", "verdict": "drift"}, Value: 1},
				{Name: "rdma_env_pre_knob_verdict", Labels: map[string]string{"device": "d1", "knob": "k1", "verdict": "converged"}, Value: 1},
			},
			"rdma_env_pre_agent_up": {
				{Name: "rdma_env_pre_agent_up", Labels: map[string]string{}, Value: 1},
			},
		},
	})
	require.Equal(t, 2, sampleCount(t, reg, "rdma_env_pre_knob_verdict"))

	// Second scrape: one knob converged (dropped), agent_up gone entirely.
	m.ExportMetrics(&collector.Info{
		Available: true,
		Series: map[string][]collector.Series{
			"rdma_env_pre_knob_verdict": {
				{Name: "rdma_env_pre_knob_verdict", Labels: map[string]string{"device": "d0", "knob": "k0", "verdict": "drift"}, Value: 1},
			},
		},
	})
	assert.Equal(t, 1, sampleCount(t, reg, "rdma_env_pre_knob_verdict"))
	assert.Equal(t, 0, sampleCount(t, reg, "rdma_env_pre_agent_up"))
}

func TestExportMetricsUnavailableClears(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := newRdmaEnvMetricsWith(reg)

	m.ExportMetrics(&collector.Info{
		Available: true,
		Series: map[string][]collector.Series{
			"rdma_env_pre_agent_up": {
				{Name: "rdma_env_pre_agent_up", Labels: map[string]string{}, Value: 1},
			},
		},
	})
	require.Equal(t, 1, sampleCount(t, reg, "rdma_env_pre_agent_up"))

	m.ExportMetrics(&collector.Info{Available: false})
	assert.Equal(t, 0, sampleCount(t, reg, "rdma_env_pre_agent_up"))
}
