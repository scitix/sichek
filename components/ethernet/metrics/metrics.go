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
	"github.com/scitix/sichek/components/ethernet/collector"
	common "github.com/scitix/sichek/metrics"
)

const (
	MetricPrefix = "sichek_ethernet"
	TagPrefix    = "json"
)

type EthernetMetrics struct {
	BondStatusGauge   *common.GaugeVecMetricExporter
	SlaveStatusGauge  *common.GaugeVecMetricExporter
	RouteStatusGauge  *common.GaugeVecMetricExporter
	TrafficStatsGauge *common.GaugeVecMetricExporter
	LACPStatusGauge   *common.GaugeVecMetricExporter
	SystemStatusGauge *common.GaugeVecMetricExporter
}

func NewEthernetMetrics() *EthernetMetrics {
	// Use distinct prefixes to avoid metric name collision.
	// We stick to ExportStruct for consistency in label cardinality.
	return &EthernetMetrics{
		BondStatusGauge:   common.NewGaugeVecMetricExporter(MetricPrefix+"_bond", []string{"bond"}),
		SlaveStatusGauge:  common.NewGaugeVecMetricExporter(MetricPrefix+"_slave", []string{"bond", "slave"}),
		RouteStatusGauge:  common.NewGaugeVecMetricExporter(MetricPrefix+"_route", nil),
		TrafficStatsGauge: common.NewGaugeVecMetricExporter(MetricPrefix+"_traffic", []string{"interface"}),
		LACPStatusGauge:   common.NewGaugeVecMetricExporter(MetricPrefix+"_lacp", []string{"bond"}),
		SystemStatusGauge: common.NewGaugeVecMetricExporter(MetricPrefix+"_system", nil),
	}
}

func (m *EthernetMetrics) ExportMetrics(info *collector.EthernetInfo) {
	if info == nil {
		return
	}

	// 1. Export Bond Status
	for bondName, bondState := range info.Bonds {
		m.BondStatusGauge.ExportStruct(bondState, []string{bondName}, TagPrefix)
	}

	// 2. Export Slave Status
	for bondName, slaves := range info.Slaves {
		for slaveName, slaveState := range slaves {
			m.SlaveStatusGauge.ExportStruct(slaveState, []string{bondName, slaveName}, TagPrefix)
		}
	}

	// 3. Export Route Status
	m.RouteStatusGauge.ExportStruct(info.Routes, []string{}, TagPrefix)

	// 4. Export Traffic Stats
	for ifaceName, stats := range info.Stats {
		m.TrafficStatsGauge.ExportStruct(stats, []string{ifaceName}, TagPrefix)
	}

	// 5. Export LACP Info
	for bondName, lacpState := range info.LACP {
		m.LACPStatusGauge.ExportStruct(lacpState, []string{bondName}, TagPrefix)
	}

	// 6. Export System Info
	m.SystemStatusGauge.SetMetric("syslog_error_count", nil, float64(len(info.SyslogErrors)))
	m.SystemStatusGauge.SetMetric("bond_count", nil, float64(len(info.BondInterfaces)))
}
