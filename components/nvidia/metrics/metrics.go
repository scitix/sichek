package metrics

import (
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/nvidia/collector"
	common "github.com/scitix/sichek/metrics"
	"github.com/scitix/sichek/pkg/utils"
)

const (
	MetricPrefix = "sichek_nvidia"
	TagPrefix    = "json"
)

type NvidiaMetrics struct {
	NvidiaDevCntGauge         *common.GaugeVecMetricExporter
	NvidiaSoftwareInfoGauge   *common.GaugeVecMetricExporter
	NvidiaCudaVerionGauge     *common.GaugeVecMetricExporter
	NvidiaDevUUIDGauge        *common.GaugeVecMetricExporter
	NvidiaDeviceGauge         *common.GaugeVecMetricExporter
	NvidiaDeviceClkEventGauge *common.GaugeVecMetricExporter
	NvidiaIBGDAStatusGauge    *common.GaugeVecMetricExporter
	NvidiaP2PStatusGauge      *common.GaugeVecMetricExporter
}

func NewNvidiaMetrics() *NvidiaMetrics {
	NvidiaDevCntGauge := common.NewGaugeVecMetricExporter(MetricPrefix, nil)
	NvidiaSoftwareInfoGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"metric_name"})
	NvidiaDevUUIDGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index", "uuid"})
	NvidiaDeviceGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index"})
	NvidiaDeviceClkEventGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index", "clock_event_reason_id"})
	NvidiaIBGDAStatusGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"status"})
	NvidiaP2PStatusGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"src_index", "dst_index"})
	return &NvidiaMetrics{
		NvidiaDevCntGauge:         NvidiaDevCntGauge,
		NvidiaSoftwareInfoGauge:   NvidiaSoftwareInfoGauge,
		NvidiaDevUUIDGauge:        NvidiaDevUUIDGauge,
		NvidiaDeviceGauge:         NvidiaDeviceGauge,
		NvidiaDeviceClkEventGauge: NvidiaDeviceClkEventGauge,
		NvidiaIBGDAStatusGauge:    NvidiaIBGDAStatusGauge,
		NvidiaP2PStatusGauge:      NvidiaP2PStatusGauge,
	}
}

func (m *NvidiaMetrics) ExportMetrics(metrics *collector.NvidiaInfo) {
	// DeviceCount
	m.NvidiaDevCntGauge.SetMetric("device_count", nil, float64(metrics.DeviceCount))
	// SoftwareInfo
	m.NvidiaSoftwareInfoGauge.ExportStructWithStrField(metrics.SoftwareInfo, []string{}, TagPrefix)

	ibgdaVal := 0.0
	if metrics.IbgdaEnable != nil {
		valOps, okOps := metrics.IbgdaEnable["EnableStreamMemOPs"]
		conditionOps := okOps && valOps == "1"

		valPeer, okPeer := metrics.IbgdaEnable["PeerMappingOverride"]
		valReg, hasReg := metrics.IbgdaEnable["RegistryDwords"]
		
		conditionPeer := (okPeer && valPeer == "1") || (hasReg && strings.Contains(valReg, "PeerMappingOverride=1"))

		if conditionOps && conditionPeer {
			ibgdaVal = 1.0
		}
	}
	m.NvidiaIBGDAStatusGauge.SetMetric("ibgda_status", []string{"enabled"}, ibgdaVal)

	if metrics.P2PStatusMatrix != nil {
		for key, supported := range metrics.P2PStatusMatrix {
			parts := strings.Split(key, "-")
			if len(parts) == 2 {
				val := 0.0
				if supported {
					val = 1.0
				}
				m.NvidiaP2PStatusGauge.SetMetric("p2p_status", []string{parts[0], parts[1]}, val)
			}
		}
	}
	for _, device := range metrics.DevicesInfo {
		deviceIdx := fmt.Sprintf("%d", device.Index)
		m.NvidiaDeviceGauge.ExportStruct(device.PCIeInfo, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.States, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.Clock, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.Power, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.Temperature, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.Utilization, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.NVLinkStates, []string{deviceIdx}, TagPrefix)
		if device.ClockEvents.IsSupported {
			m.NvidiaDeviceGauge.SetMetric("gpu_idle", []string{deviceIdx}, utils.ParseBoolToFloat(device.ClockEvents.GpuIdle))
			currentClkEvent := make(map[string]struct{})
			for _, event := range device.ClockEvents.CriticalClockEvents {
				currentClkEvent[event.Name] = struct{}{}
			}
			for _, event := range collector.CriticalClockEvents {
				if _, found := currentClkEvent[event.Name]; found {
					m.NvidiaDeviceClkEventGauge.SetMetric(event.Name, []string{deviceIdx, fmt.Sprintf("%d", event.ClockEventReasonId)}, float64(1.0))
				} else {
					m.NvidiaDeviceClkEventGauge.SetMetric(event.Name, []string{deviceIdx, fmt.Sprintf("%d", event.ClockEventReasonId)}, float64(0.0))
				}
			}
		}
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.RemappedRows, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.VolatileECC, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.AggregateECC, []string{deviceIdx}, TagPrefix)
	}
}
