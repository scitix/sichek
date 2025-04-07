package metrics

import (
	"fmt"

	"github.com/scitix/sichek/components/nvidia/collector"
)

type NvidiaMetrics struct {
	NvidiaDevCntGauge         *GaugeVecMetricExporter
	NvidiaDrvVerionGauge      *GaugeVecMetricExporter
	NvidiaCudaVerionGauge     *GaugeVecMetricExporter
	NvidiaDevUUIDGauge        *GaugeVecMetricExporter
	NvidiaDeviceGauge         *GaugeVecMetricExporter
	NvidiaDeviceClkEventGauge *GaugeVecMetricExporter
}

func NewNvidiaMetrics() *NvidiaMetrics {
	NvidiaDevCntGauge := NewGaugeVecMetricExporter("sichek_nvidia", nil)
	NvidiaDrvVersionGauge := NewGaugeVecMetricExporter("sichek_nvidia", []string{"driver_version"})
	NvidiaCudaVersionGauge := NewGaugeVecMetricExporter("sichek_nvidia", []string{"cuda_version"})
	NvidiaDevUUIDGauge := NewGaugeVecMetricExporter("sichek_nvidia", []string{"index", "uuid"})
	NvidiaDeviceGauge := NewGaugeVecMetricExporter("sichek_nvidia", []string{"index"})
	NvidiaDeviceClkEventGauge := NewGaugeVecMetricExporter("sichek_nvidia", []string{"index", "clock_event_reason_id", "description"})
	return &NvidiaMetrics{
		NvidiaDevCntGauge:         NvidiaDevCntGauge,
		NvidiaDrvVerionGauge:      NvidiaDrvVersionGauge,
		NvidiaCudaVerionGauge:     NvidiaCudaVersionGauge,
		NvidiaDevUUIDGauge:        NvidiaDevUUIDGauge,
		NvidiaDeviceGauge:         NvidiaDeviceGauge,
		NvidiaDeviceClkEventGauge: NvidiaDeviceClkEventGauge,
	}
}

func (m *NvidiaMetrics) ExportMetrics(metrics *collector.NvidiaInfo) {
	m.NvidiaDevCntGauge.SetMetric("device_count", nil, float64(metrics.DeviceCount))
	m.NvidiaDrvVerionGauge.SetMetric("driver_version", []string{metrics.SoftwareInfo.DriverVersion}, 1)
	m.NvidiaCudaVerionGauge.SetMetric("cuda_version", []string{metrics.SoftwareInfo.CUDAVersion}, 1)
	for _, device := range metrics.DevicesInfo {
		m.NvidiaDeviceGauge.ExportStruct(device.PCIeInfo, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.States, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.Clock, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.Power, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.Temperature, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.Utilization, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.NVLinkStates, []string{fmt.Sprintf("%d", device.Index)}, "json")
		if device.ClockEvents.IsSupported {
			m.NvidiaDrvVerionGauge.SetMetric("gpu_idle", []string{fmt.Sprintf("%d", device.Index)}, BoolToFloat64(device.ClockEvents.GpuIdle))
			currentClkEvent := make(map[string]struct{})
			for _, event := range device.ClockEvents.CriticalClockEvents {
				currentClkEvent[event.Name] = struct{}{}
				m.NvidiaDeviceClkEventGauge.SetMetric(event.Name, []string{fmt.Sprintf("%d", device.Index), fmt.Sprintf("%d", event.ClockEventReasonId), event.Description}, float64(1.0))
			}
			for _, event := range device.ClockEvents.CriticalClockEvents {
				currentClkEvent[event.Name] = struct{}{}
				m.NvidiaDeviceClkEventGauge.SetMetric(event.Name, []string{fmt.Sprintf("%d", device.Index), fmt.Sprintf("%d", event.ClockEventReasonId), event.Description}, float64(1.0))
			}
			for eventName := range m.NvidiaDeviceClkEventGauge.metricsMap {
				if _, found := currentClkEvent[eventName]; !found {
					m.NvidiaDeviceClkEventGauge.SetMetric(eventName, []string{fmt.Sprintf("%d", device.Index), "0", "0"}, float64(0.0))
				}
			}
		}
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.RemappedRows, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.VolatileECC, []string{fmt.Sprintf("%d", device.Index)}, "json")
		m.NvidiaDeviceGauge.ExportStruct(device.MemoryErrors.AggregateECC, []string{fmt.Sprintf("%d", device.Index)}, "json")
	}
}
