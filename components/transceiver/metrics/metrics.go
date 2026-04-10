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
	"github.com/scitix/sichek/components/transceiver/collector"
	common "github.com/scitix/sichek/metrics"
)

const (
	MetricPrefix = "sichek_transceiver"
	TagPrefix    = "json"
)

type TransceiverMetrics struct {
	ModuleGauge *common.GaugeVecMetricExporter
}

func NewTransceiverMetrics() *TransceiverMetrics {
	return &TransceiverMetrics{
		ModuleGauge: common.NewGaugeVecMetricExporter(MetricPrefix+"_module", []string{"interface", "network_type"}),
	}
}

func (m *TransceiverMetrics) ExportMetrics(info *collector.TransceiverInfo) {
	if info == nil {
		return
	}
	for i := range info.Modules {
		mod := &info.Modules[i]
		m.ModuleGauge.ExportStruct(mod, []string{mod.Interface, mod.NetworkType}, TagPrefix)
	}
}
