package metrics

import (
	"github.com/scitix/sichek/components/cpu/collector"
	common "github.com/scitix/sichek/metrics"
)

const (
	MetricPrefix = "sichek_cpu"
	TagPrefix    = "json"
)

type CpuMetrics struct {
	CpuUsageGauge   *common.GaugeVecMetricExporter
	CpuGaugeWithStr *common.GaugeVecMetricExporter
}

func NewCpuMetrics() *CpuMetrics {
	CpuUsageGauge := common.NewGaugeVecMetricExporter(MetricPrefix, nil)
	CpuGaugeWithStr := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"str_metrics"})
	return &CpuMetrics{
		CpuUsageGauge:   CpuUsageGauge,
		CpuGaugeWithStr: CpuGaugeWithStr,
	}
	common "github.com/scitix/sichek/metrics"
)

const (
	MetricPrefix = "sichek_cpu"
	TagPrefix    = "json"
)

type CpuMetrics struct {
	CpuUsageGauge   *common.GaugeVecMetricExporter
	CpuGaugeWithStr *common.GaugeVecMetricExporter
}

func NewCpuMetrics() *CpuMetrics {
	CpuUsageGauge := common.NewGaugeVecMetricExporter(MetricPrefix, nil)
	CpuGaugeWithStr := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"str_metrics"})
	return &CpuMetrics{
		CpuUsageGauge:   CpuUsageGauge,
		CpuGaugeWithStr: CpuGaugeWithStr,
	}
}

func (m *CpuMetrics) ExportMetrics(metrics *collector.CPUOutput) {
func (m *CpuMetrics) ExportMetrics(metrics *collector.CPUOutput) {
	// cpu_arch_info
	m.CpuGaugeWithStr.ExportStructWithStrField(metrics.CPUArchInfo, []string{}, TagPrefix)
	// cpu_usage_info
	m.CpuUsageGauge.ExportStruct(metrics.UsageInfo, nil, TagPrefix)
	m.CpuUsageGauge.ExportStruct(metrics.UsageInfo, nil, TagPrefix)
	// host_info
	m.CpuGaugeWithStr.ExportStructWithStrField(metrics.HostInfo, []string{}, TagPrefix)
}
