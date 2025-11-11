package metrics

import (
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/infiniband/collector"
	common "github.com/scitix/sichek/metrics"
	"github.com/scitix/sichek/pkg/utils"
)

const (
	MetricPrefix = "sichek_infiniband"
	TagPrefix    = "json"
)

type IBMetrics struct {
	IBHardWareInfoGauge    *common.GaugeVecMetricExporter
	IBNumGauge             *common.GaugeVecMetricExporter
	IBPCINumGauge          *common.GaugeVecMetricExporter
	IBHardWareInfoStrGauge *common.GaugeVecMetricExporter
	IBCounterGauge         *common.GaugeVecMetricExporter
	IBSoftWareInfoGauge    *common.GaugeVecMetricExporter
}

func NewInfinibandMetrics() *IBMetrics {
	return &IBMetrics{
		IBHardWareInfoGauge:    common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev"}),
		IBNumGauge:             common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		IBPCINumGauge:          common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		IBHardWareInfoStrGauge: common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev", "metric_name"}),
		IBCounterGauge:         common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev", "counter_name"}),
		IBSoftWareInfoGauge:    common.NewGaugeVecMetricExporter(MetricPrefix, []string{"metric_name"}),
	}
}

func (m *IBMetrics) ExportMetrics(infinibandInfo *collector.InfinibandInfo) {
	// ib_hardware_info
	infinibandInfo.RLock()
	// m.IBNumGauge.SetMetric("hca_num", nil, float64(infinibandInfo.HCAPCINum))
	// m.IBPCINumGauge.SetMetric("hca_pci_num", nil, float64(len(infinibandInfo.IBPCIDevs)))
	for _, hardWareInfo := range infinibandInfo.IBHardWareInfo {
		m.IBHardWareInfoGauge.SetMetric("phy_state", []string{hardWareInfo.IBDev}, convertState(hardWareInfo.PhyState))
		m.IBHardWareInfoGauge.SetMetric("port_state", []string{hardWareInfo.IBDev}, convertState(hardWareInfo.PortState))
		m.IBHardWareInfoGauge.SetMetric("port_speed_state", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PortSpeedState))
		m.IBHardWareInfoGauge.SetMetric("pcie_speed", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIESpeed))
		m.IBHardWareInfoGauge.SetMetric("pcie_speed_min", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIETreeSpeedMin))
		m.IBHardWareInfoGauge.SetMetric("pcie_speed_state", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIESpeedState))
		m.IBHardWareInfoGauge.SetMetric("pcie_width", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIEWidth))
		m.IBHardWareInfoGauge.SetMetric("pcie_width_min", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIETreeWidthMin))
		m.IBHardWareInfoGauge.SetMetric("pcie_width_state", []string{hardWareInfo.IBDev}, utils.ParseStringToFloat(hardWareInfo.PCIEWidthState))
		m.IBHardWareInfoGauge.SetMetric("pcie_mrr", []string{hardWareInfo.IBDev}, utils.ParseStringToFloat(hardWareInfo.PCIEMRR))
		m.IBHardWareInfoStrGauge.SetMetric("net_operstate", []string{hardWareInfo.IBDev, hardWareInfo.NetOperstate}, 1)
	}
	infinibandInfo.RUnlock()

	// ib_software_info
	m.IBSoftWareInfoGauge.ExportStructWithStrField(infinibandInfo.IBSoftWareInfo, []string{}, TagPrefix)

	// ibcounters
	for IBDev, ibCounter := range infinibandInfo.IBCounters {
		for counterName, counterValue := range ibCounter {
			m.IBCounterGauge.SetMetric("counter", []string{IBDev, counterName}, float64(counterValue))
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
