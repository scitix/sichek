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

// devPortKey identifies a (ib_dev, port) tuple in the bookkeeping maps used
// to delete prometheus series for samples that disappeared between scrapes.
// Mirrors the new prometheus label set on the per-port gauges.
type devPortKey struct {
	dev  string
	port string
}

type IBMetrics struct {
	IBHardWareInfoGauge    *common.GaugeVecMetricExporter
	IBNumGauge             *common.GaugeVecMetricExporter
	IBPCINumGauge          *common.GaugeVecMetricExporter
	IBHardWareInfoStrGauge *common.GaugeVecMetricExporter
	IBCounterGauge         *common.GaugeVecMetricExporter
	IBSoftWareInfoGauge    *common.GaugeVecMetricExporter
	// previous label sets: used by deleting series that disappeared
	prevIBDevs       map[devPortKey]struct{}
	prevCounterPairs map[devPortKey]map[string]struct{} // (ibDev, port) -> set of counter names
	prevMu           sync.Mutex
}

func NewInfinibandMetrics() *IBMetrics {
	return &IBMetrics{
		IBHardWareInfoGauge:    common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev", "port"}),
		IBNumGauge:             common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		IBPCINumGauge:          common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		IBHardWareInfoStrGauge: common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev", "port", "metric_name"}),
		IBCounterGauge:         common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ib_dev", "port", "counter_name"}),
		IBSoftWareInfoGauge:    common.NewGaugeVecMetricExporter(MetricPrefix, []string{"metric_name"}),
		prevIBDevs:             make(map[devPortKey]struct{}),
		prevCounterPairs:       make(map[devPortKey]map[string]struct{}),
	}
}

// portLabel renders a hwInfo.Port as a prometheus label value.  Legacy
// single-port records (Port==0) emit "1" so that callers querying by port
// always have a stable value.
func portLabel(port int) string {
	if port <= 0 {
		return "1"
	}
	return strconv.Itoa(port)
}

func (m *IBMetrics) ExportMetrics(infinibandInfo *collector.InfinibandInfo) {
	infinibandInfo.RLock()
	curIBDevs := make(map[devPortKey]struct{}, len(infinibandInfo.IBHardWareInfo))
	hwIndex := make(map[string]devPortKey, len(infinibandInfo.IBHardWareInfo))
	for mapKey, hw := range infinibandInfo.IBHardWareInfo {
		k := devPortKey{dev: hw.IBDev, port: portLabel(hw.Port)}
		curIBDevs[k] = struct{}{}
		hwIndex[mapKey] = k
	}
	curCounterPairs := make(map[devPortKey]map[string]struct{})
	for mapKey, counters := range infinibandInfo.IBCounters {
		k, ok := hwIndex[mapKey]
		if !ok {
			// Counters without a matching hwInfo slot: fall back to
			// treating the map key as the device with port "1".
			k = devPortKey{dev: mapKey, port: "1"}
		}
		if curCounterPairs[k] == nil {
			curCounterPairs[k] = make(map[string]struct{})
		}
		for c := range counters {
			curCounterPairs[k][c] = struct{}{}
		}
	}
	infinibandInfo.RUnlock()

	m.prevMu.Lock()
	// Delete only disappeared (ib_dev, port) series
	for prev := range m.prevIBDevs {
		if _, stillPresent := curIBDevs[prev]; stillPresent {
			continue
		}
		m.IBHardWareInfoGauge.DeleteLabelValues("phy_state", []string{prev.dev, prev.port})
		m.IBHardWareInfoGauge.DeleteLabelValues("port_state", []string{prev.dev, prev.port})
		m.IBHardWareInfoGauge.DeleteLabelValues("port_speed_state", []string{prev.dev, prev.port})
	}
	for prev, prevCounters := range m.prevCounterPairs {
		if _, stillPresent := curIBDevs[prev]; stillPresent {
			continue
		}
		for prevCounter := range prevCounters {
			m.IBCounterGauge.DeleteLabelValues("counter", []string{prev.dev, prev.port, prevCounter})
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
		port := portLabel(hardWareInfo.Port)
		m.IBHardWareInfoGauge.SetMetric("phy_state", []string{hardWareInfo.IBDev, port}, convertState(hardWareInfo.PhyState))
		m.IBHardWareInfoGauge.SetMetric("port_state", []string{hardWareInfo.IBDev, port}, convertState(hardWareInfo.PortState))
		m.IBHardWareInfoGauge.SetMetric("port_speed_state", []string{hardWareInfo.IBDev, port}, convertSpeed(hardWareInfo.PortSpeedState))
	}
	// ib_counters keyed by the same per-port hwInfo map key (<ibdev>/p<port>).
	for mapKey, ibCounter := range infinibandInfo.IBCounters {
		k, ok := hwIndex[mapKey]
		if !ok {
			k = devPortKey{dev: mapKey, port: "1"}
		}
		for counterName, counterValue := range ibCounter {
			m.IBCounterGauge.SetMetric("counter", []string{k.dev, k.port, counterName}, float64(counterValue))
		}
	}
	infinibandInfo.RUnlock()

	// ib_software_info
	m.IBSoftWareInfoGauge.ExportStructWithStrField(infinibandInfo.IBSoftWareInfo, []string{}, TagPrefix)
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
