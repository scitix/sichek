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
	"strconv"

	"github.com/scitix/sichek/components/gpuprobe/collector"
	common "github.com/scitix/sichek/metrics"
)

const MetricPrefix = "sichek_gpuprobe"

// outcomeCode maps a probe outcome to a numeric value (Prometheus stores numbers).
// 0=pass 1=fail 2=skip 3=env_err 4=exec_err 5=timeout
var outcomeCode = map[string]float64{
	collector.OutcomePass:    0,
	collector.OutcomeFail:    1,
	collector.OutcomeSkip:    2,
	collector.OutcomeEnvErr:  3,
	collector.OutcomeExecErr: 4,
	collector.OutcomeTimeout: 5,
}

type GpuProbeMetrics struct {
	statusGauge   *common.GaugeVecMetricExporter
	durationGauge *common.GaugeVecMetricExporter
}

func NewGpuProbeMetrics() *GpuProbeMetrics {
	return &GpuProbeMetrics{
		statusGauge:   common.NewGaugeVecMetricExporter(MetricPrefix, []string{"gpu", "bdf"}),
		durationGauge: common.NewGaugeVecMetricExporter(MetricPrefix, []string{"gpu", "bdf"}),
	}
}

func (m *GpuProbeMetrics) ExportMetrics(info *collector.GpuProbeInfo) {
	for _, g := range info.PerGPU {
		gpu := strconv.Itoa(g.Index)
		m.statusGauge.SetMetric("probe_status", []string{gpu, g.BDF}, outcomeCode[g.Outcome])
		m.durationGauge.SetMetric("duration_ms", []string{gpu, g.BDF}, float64(g.DurationMs))
	}
}
