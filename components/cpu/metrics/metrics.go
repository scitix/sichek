package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/cpu/collector"
)

var (
	cpuGaugeMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_gauge_metrics",
		},
		[]string{"component_name", "metric_name"},
	)
	cpuGaugeMetricsWithValues = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_gauge_metrics_with_values",
		},
		[]string{"component_name", "metric_name", "value"},
	)
)

func InitCpuMetrics() {
	prometheus.MustRegister(cpuGaugeMetrics)
	prometheus.MustRegister(cpuGaugeMetricsWithValues)
}

func ExportCPUMetrics(metrics *collector.CPUOutput) {
	// cpu_arch_info
	cpuGaugeMetricsWithValues.WithLabelValues("cpu", "architecture", metrics.CPUArchInfo.Architecture).Set(1)
	cpuGaugeMetricsWithValues.WithLabelValues("cpu", "model_name", metrics.CPUArchInfo.ModelName).Set(1)
	cpuGaugeMetricsWithValues.WithLabelValues("cpu", "vendor_id", metrics.CPUArchInfo.VendorID).Set(1)
	cpuGaugeMetricsWithValues.WithLabelValues("cpu", "family", metrics.CPUArchInfo.Family).Set(1)
	cpuGaugeMetrics.WithLabelValues("cpu", "socket").Set(float64(metrics.CPUArchInfo.Sockets))
	cpuGaugeMetrics.WithLabelValues("cpu", "cores_per_socket").Set(float64(metrics.CPUArchInfo.CoresPerSocket))
	cpuGaugeMetrics.WithLabelValues("cpu", "threads_per_core").Set(float64(metrics.CPUArchInfo.ThreadPerCore))
	cpuGaugeMetrics.WithLabelValues("cpu", "numa_num").Set(float64(metrics.CPUArchInfo.NumaNum))

	// cpu_usage_info
	cpuGaugeMetrics.WithLabelValues("cpu", "runnable_task_count").Set(float64(metrics.UsageInfo.RunnableTaskCount))
	cpuGaugeMetrics.WithLabelValues("cpu", "total_thread_count").Set(float64(metrics.UsageInfo.TotalThreadCount))
	cpuGaugeMetrics.WithLabelValues("cpu", "usage_time").Set(float64(metrics.UsageInfo.CPUUsageTime))
	cpuGaugeMetrics.WithLabelValues("cpu", "cpu_load_avg1m").Set(float64(metrics.UsageInfo.CpuLoadAvg1m))
	cpuGaugeMetrics.WithLabelValues("cpu", "cpu_load_avg5m").Set(float64(metrics.UsageInfo.CpuLoadAvg5m))
	cpuGaugeMetrics.WithLabelValues("cpu", "cpu_load_avg15m").Set(float64(metrics.UsageInfo.CpuLoadAvg15m))
	cpuGaugeMetrics.WithLabelValues("cpu", "system_processes_total").Set(float64(metrics.UsageInfo.SystemProcessesTotal))
	cpuGaugeMetrics.WithLabelValues("cpu", "system_procs_running").Set(float64(metrics.UsageInfo.SystemProcsRunning))
	cpuGaugeMetrics.WithLabelValues("cpu", "system_procs_blocked").Set(float64(metrics.UsageInfo.SystemProcsBlocked))
	cpuGaugeMetrics.WithLabelValues("cpu", "system_interrupts_total").Set(float64(metrics.UsageInfo.SystemInterruptsTotal))

	// host_info
	cpuGaugeMetricsWithValues.WithLabelValues("cpu", "hostname", metrics.HostInfo.Hostname).Set(1)
	cpuGaugeMetricsWithValues.WithLabelValues("cpu", "os_version", metrics.HostInfo.OSVersion).Set(1)
	cpuGaugeMetricsWithValues.WithLabelValues("cpu", "kernel_version", metrics.HostInfo.KernelVersion).Set(1)
}
