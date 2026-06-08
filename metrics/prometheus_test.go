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
	"github.com/stretchr/testify/assert"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

// countSeries collects all currently-set series from a registered GaugeVec.
func countSeries(t *testing.T, m *HealthCheckResMetrics, metricName string) int {
	t.Helper()
	fullName := sanitizeMetricName(MetricPrefix + "_" + metricName)
	gv, ok := m.HealthCheckResGauge.MetricsMap[fullName]
	if !ok {
		return 0
	}
	ch := make(chan prometheus.Metric, 1024)
	gv.Collect(ch)
	close(ch)
	return len(ch)
}

// A single failed device whose string happens to contain the per-device join
// delimiter (",") must still produce exactly one Prometheus series, not be cut
// into fragments. This guards against the PCIe-tree-width/speed regression where
// "mlx5_4(0000:9a:00.0, bottleneck@...)" was split into two bogus series.
func TestExportMetrics_SingleDeviceNotSplit(t *testing.T) {
	m := newHealthCheckResMetrics()
	const errorName = "PCIeTreeWidthIncorrectSingleTest"
	res := &common.Result{
		Item: "infiniband",
		Checkers: []*common.CheckerResult{
			{
				Name:      "check_pcie_tree_width",
				Status:    consts.StatusAbnormal,
				ErrorName: errorName,
				// Post-fix format: no comma inside a single device string.
				Device: "mlx5_4(0000:9a:00.0 bottleneck@0000:97:01.0->0000:98:00.0)",
				Curr:   "1",
				Spec:   "16",
				Level:  consts.LevelCritical,
			},
		},
	}
	m.ExportMetrics(res)
	assert.Equal(t, 1, countSeries(t, m, "infiniband_"+errorName))
}

// When several distinct devices fail, the Device field is comma-joined and the
// exporter must split it back into one series per device.
func TestExportMetrics_MultipleDevicesSplit(t *testing.T) {
	m := newHealthCheckResMetrics()
	const errorName = "PCIeTreeWidthIncorrectMultiTest"
	res := &common.Result{
		Item: "infiniband",
		Checkers: []*common.CheckerResult{
			{
				Name:      "check_pcie_tree_width",
				Status:    consts.StatusAbnormal,
				ErrorName: errorName,
				Device: "mlx5_4(0000:9a:00.0 bottleneck@0000:97:01.0->0000:98:00.0)," +
					"mlx5_5(0000:9b:00.0 bottleneck@0000:97:01.0->0000:99:00.0)",
				Curr:  "1",
				Spec:  "16",
				Level: consts.LevelCritical,
			},
		},
	}
	m.ExportMetrics(res)
	assert.Equal(t, 2, countSeries(t, m, "infiniband_"+errorName))
}
