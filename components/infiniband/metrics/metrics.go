package metrics

import (
	"strconv"
	"strings"
	"sync"

	"github.com/scitix/sichek/components/infiniband/collector"
	common "github.com/scitix/sichek/metrics"
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
	// previous label sets: used by deleting series that disappeared
	prevIBDevs       map[string]struct{}
	prevCounterPairs map[string]map[string]struct{} // ibDev -> set of counter names
	prevMu           sync.Mutex
}

func NewInfinibandMetrics() *IBMetrics {
	return &IBMetrics{
		IBHardWareInfoGauge:    common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev"}),
		IBNumGauge:             common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		IBPCINumGauge:          common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		IBHardWareInfoStrGauge: common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev", "metric_name"}),
		IBCounterGauge:         common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev", "counter_name"}),
		IBSoftWareInfoGauge:    common.NewGaugeVecMetricExporter(MetricPrefix, []string{"metric_name"}),
		prevIBDevs:             make(map[string]struct{}),
		prevCounterPairs:       make(map[string]map[string]struct{}),
	}
}

func (m *IBMetrics) ExportMetrics(infinibandInfo *collector.InfinibandInfo) {
	infinibandInfo.RLock()
	curIBDevs := make(map[string]struct{}, len(infinibandInfo.IBHardWareInfo))
	for ibDev := range infinibandInfo.IBHardWareInfo {
		curIBDevs[ibDev] = struct{}{}
	}
	curCounterPairs := make(map[string]map[string]struct{})
	for ibDev, counters := range infinibandInfo.IBCounters {
		if curCounterPairs[ibDev] == nil {
			curCounterPairs[ibDev] = make(map[string]struct{})
		}
		for c := range counters {
			curCounterPairs[ibDev][c] = struct{}{}
		}
	}
	infinibandInfo.RUnlock()

	m.prevMu.Lock()
	// Delete only disappeared device series
	for prevDev := range m.prevIBDevs {
		if _, stillPresent := curIBDevs[prevDev]; stillPresent {
			continue
		}
		m.IBHardWareInfoGauge.DeleteLabelValues("phy_state", []string{prevDev})
		m.IBHardWareInfoGauge.DeleteLabelValues("port_state", []string{prevDev})
		m.IBHardWareInfoGauge.DeleteLabelValues("port_speed_state", []string{prevDev})
	}
	for prevDev, prevCounters := range m.prevCounterPairs {
		if _, stillPresent := curIBDevs[prevDev]; stillPresent {
			continue
		}
		for prevCounter := range prevCounters {
			m.IBCounterGauge.DeleteLabelValues("counter", []string{prevDev, prevCounter})
		}
	}
	m.prevIBDevs = curIBDevs
	m.prevCounterPairs = curCounterPairs
	m.prevMu.Unlock()

	// ib_hardware_info
	infinibandInfo.RLock()
	m.IBNumGauge.SetMetric("hca_num", nil, float64(infinibandInfo.HCAPCINum))
	m.IBPCINumGauge.SetMetric("hca_pci_num", nil, float64(len(infinibandInfo.IBPCIDevs)))
	for _, hardWareInfo := range infinibandInfo.IBHardWareInfo {
		m.IBHardWareInfoGauge.SetMetric("phy_state", []string{hardWareInfo.IBDev}, convertState(hardWareInfo.PhyState))
		m.IBHardWareInfoGauge.SetMetric("port_state", []string{hardWareInfo.IBDev}, convertState(hardWareInfo.PortState))
		m.IBHardWareInfoGauge.SetMetric("port_speed_state", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PortSpeedState))
		// m.IBHardWareInfoGauge.SetMetric("pcie_speed", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIESpeed))
		// m.IBHardWareInfoGauge.SetMetric("pcie_speed_min", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIETreeSpeedMin))
		// m.IBHardWareInfoGauge.SetMetric("pcie_speed_state", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIESpeedState))
		// m.IBHardWareInfoGauge.SetMetric("pcie_width", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIEWidth))
		// m.IBHardWareInfoGauge.SetMetric("pcie_width_min", []string{hardWareInfo.IBDev}, convertSpeed(hardWareInfo.PCIETreeWidthMin))
		// m.IBHardWareInfoGauge.SetMetric("pcie_width_state", []string{hardWareInfo.IBDev}, utils.ParseStringToFloat(hardWareInfo.PCIEWidthState))
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

func extractFirstNumber(s string) float64 {
	var firstPart string

	if idx := strings.IndexAny(s, " :"); idx != -1 {
		firstPart = s[:idx]
	} else {
		firstPart = s
	}

	value, err := strconv.ParseFloat(firstPart, 64)
	if err != nil {
		return 0
	}
	return value
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
