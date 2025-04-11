package metrics

import (
	"fmt"

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
}

func NewNvidiaMetrics() *NvidiaMetrics {
	NvidiaDevCntGauge := common.NewGaugeVecMetricExporter(MetricPrefix, nil)
	NvidiaSoftwareInfoGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"metric_name"})
	NvidiaDevUUIDGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index", "uuid"})
	NvidiaDeviceGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index"})
	NvidiaDeviceClkEventGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index", "clock_event_reason_id", "description"})
	return &NvidiaMetrics{
		NvidiaDevCntGauge:         NvidiaDevCntGauge,
		NvidiaSoftwareInfoGauge:   NvidiaSoftwareInfoGauge,
		NvidiaDevUUIDGauge:        NvidiaDevUUIDGauge,
		NvidiaDeviceGauge:         NvidiaDeviceGauge,
		NvidiaDeviceClkEventGauge: NvidiaDeviceClkEventGauge,
	}
}

func (m *NvidiaMetrics) ExportMetrics(metrics *collector.NvidiaInfo) {
	// DeviceCount
	m.NvidiaDevCntGauge.SetMetric("device_count", nil, float64(metrics.DeviceCount))
	// SoftwareInfo
	m.NvidiaSoftwareInfoGauge.ExportStructWithStrField(metrics.SoftwareInfo, []string{}, TagPrefix)

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
				m.NvidiaDeviceClkEventGauge.SetMetric(event.Name, []string{deviceIdx, fmt.Sprintf("%d", event.ClockEventReasonId), event.Description}, float64(1.0))
			}
			for _, event := range device.ClockEvents.CriticalClockEvents {
				currentClkEvent[event.Name] = struct{}{}
				m.NvidiaDeviceClkEventGauge.SetMetric(event.Name, []string{deviceIdx, fmt.Sprintf("%d", event.ClockEventReasonId), event.Description}, float64(1.0))
			}
			for eventName := range m.NvidiaDeviceClkEventGauge.MetricsMap {
				if _, found := currentClkEvent[eventName]; !found {
					m.NvidiaDeviceClkEventGauge.SetMetric(eventName, []string{deviceIdx, "0", "0"}, float64(0.0))
				}
			}
		}
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.RemappedRows, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.VolatileECC, []string{deviceIdx}, TagPrefix)
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.AggregateECC, []string{deviceIdx}, TagPrefix)
	}
}
