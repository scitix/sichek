package metrics

import (
	"github.com/scitix/sichek/components/cpu/collector"
	common "github.com/scitix/sichek/metrics"
	"github.com/scitix/sichek/pkg/utils"
)

const (
	MetricPrefix = "sichek_cpu"
	TagPrefix    = "json"
)

// uncore domain metric names; every series carries the domain and interface labels.
var uncoreDomainMetrics = []string{
	"uncore_min_freq_khz",
	"uncore_max_freq_khz",
	"uncore_initial_max_freq_khz",
	"uncore_current_freq_khz",
	"uncore_pinned",
}

type CpuMetrics struct {
	CpuUsageGauge   *common.GaugeVecMetricExporter
	CpuGaugeWithStr *common.GaugeVecMetricExporter
	UncoreGauge     *common.GaugeVecMetricExporter
	UncoreDomGauge  *common.GaugeVecMetricExporter
}

func NewCpuMetrics() *CpuMetrics {
	CpuUsageGauge := common.NewGaugeVecMetricExporter(MetricPrefix, nil)
	CpuGaugeWithStr := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"str_metrics"})
	return &CpuMetrics{
		CpuUsageGauge:   CpuUsageGauge,
		CpuGaugeWithStr: CpuGaugeWithStr,
		UncoreGauge:     common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		UncoreDomGauge:  common.NewGaugeVecMetricExporter(MetricPrefix, []string{"domain", "interface"}),
	}
}

func (m *CpuMetrics) ExportMetrics(metrics *collector.CPUOutput) {
	// cpu_arch_info
	m.CpuGaugeWithStr.ExportStructWithStrField(metrics.CPUArchInfo, []string{}, TagPrefix)
	// cpu_usage_info
	m.CpuUsageGauge.ExportStruct(metrics.UsageInfo, nil, TagPrefix)
	// host_info
	m.CpuGaugeWithStr.ExportStructWithStrField(metrics.HostInfo, []string{}, TagPrefix)
	// uncore frequency
	m.exportUncore(&metrics.UncoreInfo)
}

func (m *CpuMetrics) exportUncore(u *collector.UncoreInfo) {
	if !u.Applicable {
		return
	}
	m.UncoreGauge.SetMetric("uncore_driver_loaded", nil, utils.ParseBoolToFloat(u.DriverLoaded))
	// Reset first so domains that disappear (driver unloaded / interface switch) drop their series.
	for _, name := range uncoreDomainMetrics {
		m.UncoreDomGauge.ResetMetric(name)
	}
	for _, d := range u.Domains {
		labels := []string{d.Name, u.Interface}
		m.UncoreDomGauge.SetMetric("uncore_min_freq_khz", labels, float64(d.MinFreqKHz))
		m.UncoreDomGauge.SetMetric("uncore_max_freq_khz", labels, float64(d.MaxFreqKHz))
		m.UncoreDomGauge.SetMetric("uncore_initial_max_freq_khz", labels, float64(d.InitialMaxFreqKHz))
		m.UncoreDomGauge.SetMetric("uncore_current_freq_khz", labels, float64(d.CurrentFreqKHz))
		m.UncoreDomGauge.SetMetric("uncore_pinned", labels, utils.ParseBoolToFloat(d.Pinned()))
	}
}
