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
	EthernetDevGauge          *common.GaugeVecMetricExporter
	EthernetHardWareInfoGauge *common.GaugeVecMetricExporter
}

func NewEthernetMetrics() *EthernetMetrics {
	EthernetDevGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"device_name"})
	EthernetHardWareInfoGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"eth_dev", "phy_stat"})
	return &EthernetMetrics{
		EthernetDevGauge:          EthernetDevGauge,
		EthernetHardWareInfoGauge: EthernetHardWareInfoGauge,
	}
}
func (m *EthernetMetrics) ExportMetrics(metrics *collector.EthernetInfo) {
	for _, device := range metrics.EthDevs {
		m.EthernetDevGauge.SetMetric("eth_dev", []string{device}, float64(1.0))
	}
	for _, hardWareInfo := range metrics.EthHardWareInfo {
		m.EthernetHardWareInfoGauge.SetMetric("eth_dev", []string{hardWareInfo.EthDev, hardWareInfo.PhyState}, float64(1.0))
	}
}
