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
	dto "github.com/prometheus/client_model/go"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// collectVBIOS returns index -> vbios_version for every exported series.
func collectVBIOS(t *testing.T, g *prometheus.GaugeVec) map[string]string {
	ch := make(chan prometheus.Metric, 16)
	g.Collect(ch)
	close(ch)
	got := map[string]string{}
	for m := range ch {
		var pb dto.Metric
		require.NoError(t, m.Write(&pb))
		assert.Equal(t, float64(1), pb.GetGauge().GetValue())
		labels := map[string]string{}
		for _, l := range pb.GetLabel() {
			labels[l.GetName()] = l.GetValue()
		}
		got[labels["index"]] = labels["vbios_version"]
	}
	return got
}

func TestExportMetrics_VBIOSVersion(t *testing.T) {
	m := NewNvidiaMetrics()
	m.ExportMetrics(&collector.NvidiaInfo{DevicesInfo: []collector.DeviceInfo{
		{Index: 0, VBIOSVersion: "97.00.66.00.01"},
		{Index: 1, VBIOSVersion: ""},
	}})
	gauge := m.NvidiaVBIOSGauge.MetricsMap["sichek_nvidia_vbios_version"]
	require.NotNil(t, gauge)
	assert.Equal(t, map[string]string{"0": "97.00.66.00.01"}, collectVBIOS(t, gauge), "GPU with empty vbios must not emit a series")

	// After a reflash the old version label must disappear.
	m.ExportMetrics(&collector.NvidiaInfo{DevicesInfo: []collector.DeviceInfo{
		{Index: 0, VBIOSVersion: "97.00.88.00.0F"},
	}})
	assert.Equal(t, map[string]string{"0": "97.00.88.00.0F"}, collectVBIOS(t, gauge))
}
