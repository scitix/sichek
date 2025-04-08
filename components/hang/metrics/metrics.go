package metrics

import (
	"github.com/scitix/sichek/components/hang/checker"
	common "github.com/scitix/sichek/metrics"
)

const (
	MetricPrefix = "sichek_hang"
	TagPrefix    = "json"
)

type HangMetrics struct {
	HangGauge *common.GaugeVecMetricExporter
}

func NewHangMetrics() *HangMetrics {
	HangGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"name","indicate_name"})
	return &HangMetrics{
		HangGauge: HangGauge,
	}
}

func (m *HangMetrics) ExportMetrics(hangInfo *checker.HangInfo) {
	for indicateName, name2duration := range hangInfo.HangDuration {
		for name, duration := range name2duration {
			m.HangGauge.SetMetric("duration", []string{name, indicateName}, float64(duration))
			m.HangGauge.SetMetric("threshold", []string{name, indicateName}, float64(hangInfo.HangThreshold[indicateName]))
		}
	}
}
