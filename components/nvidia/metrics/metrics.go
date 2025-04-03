package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/pkg/utils"
)

var (
	NvidiaGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nvidia_metrics",
		},
		[]string{"component_name", "metric_name"},
	)
	NvidiaValueGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nvidia_value_metrics",
		},
		[]string{"component_name", "metric_name", "value"},
	)
	NvidiaDeviceGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nvidia_device_metrics",
		},
		[]string{"component_name", "device_index", "metric_name"},
	)
	NvidiaNVLinkGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nvidia_nvlink_metrics",
		},
		[]string{"component_name", "device_index", "link_no", "metric_name"},
	)
)

func InitNvidiaMetrics() {
	prometheus.MustRegister(NvidiaGauge)
	prometheus.MustRegister(NvidiaValueGauge)
	prometheus.MustRegister(NvidiaNVLinkGauge)
}

func ExportNvidiaMetrics(metrics *collector.NvidiaInfo) {
	NvidiaGauge.WithLabelValues("nvidia", "device_count").Set(float64(metrics.DeviceCount))
	NvidiaValueGauge.WithLabelValues("nvidia", "driver_version", metrics.SoftwareInfo.DriverVersion).Set(1)
	NvidiaValueGauge.WithLabelValues("nvidia", "cuda_version", metrics.SoftwareInfo.CUDAVersion).Set(1)
	for _, device := range metrics.DevicesInfo {
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "pci_gen").Set(float64(device.PCIeInfo.PCILinkGen))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "pci_gen_max").Set(float64(device.PCIeInfo.PCILinkGenMAX))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "pci_width").Set(float64(device.PCIeInfo.PCILinkWidth))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "pci_width_max").Set(float64(device.PCIeInfo.PCILinkWidthMAX))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "pci_tx").Set(float64(device.PCIeInfo.PCIeTx))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "pci_rx").Set(float64(device.PCIeInfo.PCIeRx))

		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "power_usage").Set(float64(device.Power.PowerUsage))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "seted_power_limit_W").Set(float64(device.Power.CurPowerLimit))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "default_power_limit_W").Set(float64(device.Power.DefaultPowerLimit))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "enforced_power_limit_W").Set(float64(device.Power.EnforcedPowerLimit))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "min_power_limit_W").Set(float64(device.Power.MinPowerLimit))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "max_power_limit_W").Set(float64(device.Power.MaxPowerLimit))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "power_violations").Set(float64(device.Power.PowerViolations))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "thermal_violations").Set(float64(device.Power.ThermalViolations))

		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "current_temperature_C").Set(float64(device.Temperature.GPUCurTemperature))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "threadhold_temperature_C").Set(float64(device.Temperature.GPUThresholdTemperature))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "threadhold_temperature_shutdown_C").Set(float64(device.Temperature.GPUThresholdTemperatureShutdown))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "threadhold_temperature_slowdown_C").Set(float64(device.Temperature.GPUThresholdTemperatureSlowdown))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "current_memory_temperature_C").Set(float64(device.Temperature.MemoryCurTemperature))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "max_memory_operation_temperature_C").Set(float64(device.Temperature.MemoryMaxOperationLimitTemperature))

		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "gpu_usage_percent").Set(float64(device.Utilization.GPUUsagePercent))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "gpu_memory_usage_percent").Set(float64(device.Utilization.MemoryUsagePercent))

		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "nvlink_supported").Set(utils.ParseBoolToFloat(device.NVLinkStates.NVlinkSupported))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "all_feature_enabled").Set(utils.ParseBoolToFloat(device.NVLinkStates.AllFeatureEnabled))
		NvidiaDeviceGauge.WithLabelValues("nvidia", device.UUID, "active_nvlink_num").Set(float64(device.NVLinkStates.NvlinkNum))
		for _, nvlinkState := range device.NVLinkStates.NVLinkStates {
			NvidiaNVLinkGauge.WithLabelValues("nvidia", device.UUID, strconv.Itoa(nvlinkState.LinkNo), "nvlink_supported").Set(utils.ParseBoolToFloat(nvlinkState.NVlinkSupported))
			NvidiaNVLinkGauge.WithLabelValues("nvidia", device.UUID, strconv.Itoa(nvlinkState.LinkNo), "feature_enabled").Set(utils.ParseBoolToFloat(nvlinkState.FeatureEnabled))
			NvidiaNVLinkGauge.WithLabelValues("nvidia", device.UUID, strconv.Itoa(nvlinkState.LinkNo), "throughput_raw_rx_bytes").Set(float64(nvlinkState.ThroughputRawRxBytes))
			NvidiaNVLinkGauge.WithLabelValues("nvidia", device.UUID, strconv.Itoa(nvlinkState.LinkNo), "throughput_raw_tx_bytes").Set(float64(nvlinkState.ThroughputRawTxBytes))
		}

	}

}
