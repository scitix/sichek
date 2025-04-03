package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/ethernet/collector"
)

var (
	ethernetGaugeMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ethernet_gauge_metrics",
		},
		[]string{"component_name", "metric_name", "dev_name", "phy_state"},
	)
)

func InitEthernetMetrics() {
	prometheus.MustRegister(ethernetGaugeMetrics)
}

func ExportEthernetMetrics(ethernetInfo *collector.EthernetInfo) {
	// eth_hardware_info
	for _, eth := range ethernetInfo.EthHardWareInfo {
		ethernetGaugeMetrics.WithLabelValues("ethernet", "hardware_info", eth.EthDev, eth.PhyState).Set(1)
	}
}
