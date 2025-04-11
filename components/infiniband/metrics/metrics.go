package metrics

import (
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/infiniband/collector"
	common "github.com/scitix/sichek/metrics"
	"github.com/scitix/sichek/pkg/utils"
)

var (
	infinibandGaugeMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "infiniband_gauge_metrics",
		},
		[]string{"component_name", "metric_name"},
	)
	ibDevGaugeMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "infiniband_dev_gauge_metrics",
		},
		[]string{"component_name", "dev_name", "metric_name"},
	)
	ibDevGaugeMetricsWithValues = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "infiniband_dev_gauge_metrics_with_values",
		},
		[]string{"component_name", "dev_name", "metric_name", "metric_value"},
	)
	MetricPrefix = "sichek_ib"
)

type IBMetrics struct {
	NvidiaDevCntGauge         *common.GaugeVecMetricExporter
	NvidiaSoftwareInfoGauge   *common.GaugeVecMetricExporter
	NvidiaCudaVerionGauge     *common.GaugeVecMetricExporter
	NvidiaDevUUIDGauge        *common.GaugeVecMetricExporter
	NvidiaDeviceGauge         *common.GaugeVecMetricExporter
	NvidiaDeviceClkEventGauge *common.GaugeVecMetricExporter
}

func InitInfinibandMetrics() *IBMetrics {
	prometheus.MustRegister(infinibandGaugeMetrics)
	prometheus.MustRegister(ibDevGaugeMetrics)
	prometheus.MustRegister(ibDevGaugeMetricsWithValues)

	return &IBMetrics{
		NvidiaDevCntGauge:         common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		NvidiaSoftwareInfoGauge:   common.NewGaugeVecMetricExporter(MetricPrefix, []string{"metric_name"}),
		NvidiaDevUUIDGauge:        common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index", "uuid"}),
		NvidiaDeviceGauge:         common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index"}),
		NvidiaDeviceClkEventGauge: common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index", "clock_event_reason_id", "description"}),
	}
}

func ExportInfinibandMetrics(infinibandInfo *collector.InfinibandInfo) {
	infinibandGaugeMetrics.WithLabelValues("infiniband", "hca_pci_num").Set(float64(infinibandInfo.HCAPCINum))
	//ib_hardware_info
	for _, hardWareInfo := range infinibandInfo.IBHardWareInfo {

		ibDevGaugeMetrics.WithLabelValues("infiniband", hardWareInfo.IBDev, "phy_state").Set(convertState(hardWareInfo.PhyState))
		ibDevGaugeMetrics.WithLabelValues("infiniband", hardWareInfo.IBDev, "port_state").Set(convertState(hardWareInfo.PortState))
		ibDevGaugeMetrics.WithLabelValues("infiniband", hardWareInfo.IBDev, "port_speed").Set(convertSpeed(hardWareInfo.PortSpeed))
		ibDevGaugeMetrics.WithLabelValues("infiniband", hardWareInfo.IBDev, "pcie_speed").Set(convertSpeed(hardWareInfo.PCIESpeed))
		ibDevGaugeMetrics.WithLabelValues("infiniband", hardWareInfo.IBDev, "pcie_width").Set(utils.ParseStringToFloat(hardWareInfo.PCIEWidth))
		ibDevGaugeMetrics.WithLabelValues("infiniband", hardWareInfo.IBDev, "pcie_mrr").Set(utils.ParseStringToFloat(hardWareInfo.PCIEMRR))

		ibDevGaugeMetricsWithValues.WithLabelValues("infiniband", hardWareInfo.IBDev, "net_operstate", hardWareInfo.NetOperstate).Set(1)
		ibDevGaugeMetricsWithValues.WithLabelValues("infiniband", hardWareInfo.IBDev, "fw_ver", hardWareInfo.FWVer).Set(1)
		ibDevGaugeMetricsWithValues.WithLabelValues("infiniband", hardWareInfo.IBDev, "ofed_ver", hardWareInfo.FWVer).Set(1)
	}
	//ib_software_info
	ibDevGaugeMetricsWithValues.WithLabelValues("infiniband", "ib_software_info", "ofed_ver", infinibandInfo.IBSoftWareInfo.OFEDVer).Set(1)

	// ibcounters
	for IBDev, ibCounter := range infinibandInfo.IBCounters {
		for counterName, counterValue := range ibCounter {
			ibDevGaugeMetrics.WithLabelValues("infiniband", IBDev, counterName).Set(float64(counterValue))
		}
	}

}
func convertState(state string) float64 {
	stateStr := strings.Split(state, ":")
	convertedState, err := strconv.ParseFloat(stateStr[0], 64)
	if err != nil {
		return 0
	}
	return convertedState
}
func convertSpeed(portSpeed string) float64 {
	portSpeedStr := strings.Split(portSpeed, " ")
	convertedPortSpeed, err := strconv.ParseFloat(portSpeedStr[0], 64)
	if err != nil {
		return 0

	}
	return convertedPortSpeed
}
