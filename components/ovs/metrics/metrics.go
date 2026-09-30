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
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	common "github.com/scitix/sichek/metrics"
)

const MetricPrefix = "sichek_ovs"

type OVSMetrics struct {
	expectedOtherConfig map[string]string // for other_config_ok comparison

	present    *common.GaugeVecMetricExporter // no extra labels
	serviceUp  *common.GaugeVecMetricExporter // {ovs_service}
	bridge     *common.GaugeVecMetricExporter // {bridge} -> flow/group/port/issue counts via metric name
	otherCfgOK *common.GaugeVecMetricExporter // {key}
	dpLookup   *common.GaugeVecMetricExporter // {datapath, result}
	dpFlows    *common.GaugeVecMetricExporter // {datapath}
	pmd        *common.GaugeVecMetricExporter // {core, numa}
	coverage   *common.GaugeVecMetricExporter // {event}
	port       *common.GaugeVecMetricExporter // {bridge, port}
	portDir    *common.GaugeVecMetricExporter // {bridge, port, direction}
}

func NewOVSMetrics() *OVSMetrics {
	return &OVSMetrics{
		expectedOtherConfig: config.DefaultSpec().OtherConfig,
		present:             common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		serviceUp:           common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ovs_service"}),
		bridge:              common.NewGaugeVecMetricExporter(MetricPrefix, []string{"bridge"}),
		otherCfgOK:          common.NewGaugeVecMetricExporter(MetricPrefix, []string{"key"}),
		dpLookup:            common.NewGaugeVecMetricExporter(MetricPrefix, []string{"datapath", "result"}),
		dpFlows:             common.NewGaugeVecMetricExporter(MetricPrefix, []string{"datapath"}),
		pmd:                 common.NewGaugeVecMetricExporter(MetricPrefix, []string{"core", "numa"}),
		coverage:            common.NewGaugeVecMetricExporter(MetricPrefix, []string{"event"}),
		port:                common.NewGaugeVecMetricExporter(MetricPrefix, []string{"bridge", "port"}),
		portDir:             common.NewGaugeVecMetricExporter(MetricPrefix, []string{"bridge", "port", "direction"}),
	}
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (m *OVSMetrics) ExportMetrics(info *collector.OVSInfo) {
	if info == nil {
		return
	}
	m.present.SetMetric("present", nil, b2f(info.Available))
	if !info.Available {
		return
	}
	for svc, state := range info.Services {
		m.serviceUp.SetMetric("service_up", []string{svc}, b2f(state == "active"))
	}
	for _, b := range info.Bridges {
		m.bridge.SetMetric("bridge_flow_count", []string{b.Name}, float64(b.Flows))
		m.bridge.SetMetric("bridge_group_count", []string{b.Name}, float64(len(b.GroupIDs)))
		m.bridge.SetMetric("bridge_port_count", []string{b.Name}, float64(b.Ports))
		m.bridge.SetMetric("bridge_issue_count", []string{b.Name}, float64(len(b.OrphanFlowRefs)+len(b.OrphanPorts)))

		for _, p := range b.PortDetails {
			m.port.SetMetric("port_ofport", []string{b.Name, p.Name}, float64(p.OFPort))
			m.port.SetMetric("port_rx_errors", []string{b.Name, p.Name}, float64(p.RxErrPkts))
			m.portDir.SetMetric("port_bytes", []string{b.Name, p.Name, "rx"}, float64(p.RxBytes))
			m.portDir.SetMetric("port_bytes", []string{b.Name, p.Name, "tx"}, float64(p.TxBytes))
		}
	}
	for k, want := range m.expectedOtherConfig {
		m.otherCfgOK.SetMetric("other_config_ok", []string{k}, b2f(info.OtherConfig[k] == want))
	}
	m.dpFlows.SetMetric("datapath_flows", []string{info.Datapath.Name}, float64(info.Datapath.DPFlows))
	m.dpLookup.SetMetric("datapath_lookup", []string{info.Datapath.Name, "hit"}, float64(info.Datapath.LookupsHit))
	m.dpLookup.SetMetric("datapath_lookup", []string{info.Datapath.Name, "missed"}, float64(info.Datapath.LookupsMissed))
	m.dpLookup.SetMetric("datapath_lookup", []string{info.Datapath.Name, "lost"}, float64(info.Datapath.LookupsLost))
	for _, p := range info.Datapath.PMDs {
		m.pmd.SetMetric("pmd_busy_ratio", []string{p.Core, p.NUMA}, p.BusyRatio)
		m.pmd.SetMetric("pmd_idle_cycles", []string{p.Core, p.NUMA}, float64(p.IdleCycles))
		m.pmd.SetMetric("pmd_processing_cycles", []string{p.Core, p.NUMA}, float64(p.ProcessingCycles))
		m.pmd.SetMetric("pmd_rx_packets", []string{p.Core, p.NUMA}, float64(p.RxPackets))
	}
	for ev, total := range info.Coverage {
		m.coverage.SetMetric("coverage_total", []string{ev}, float64(total))
	}
}
