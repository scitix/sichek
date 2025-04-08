package metrics

import (
	"github.com/scitix/sichek/components/memory/collector"
	common "github.com/scitix/sichek/metrics"
)

const (
	MetricPrefix = "sichek_memory"
	TagPrefix    = "json"
)

type MemoryMetrics struct {
	MemoryGauge *common.GaugeVecMetricExporter
}

func NewMemoryMetrics() *MemoryMetrics {
	MemoryGauge := common.NewGaugeVecMetricExporter(MetricPrefix, nil)
	return &MemoryMetrics{
		MemoryGauge: MemoryGauge,
	}
}

func (m *MemoryMetrics) ExportMetrics(metrics *collector.MemoryInfo) {
	m.MemoryGauge.ExportStructWithStrField(metrics, nil, TagPrefix)
}
