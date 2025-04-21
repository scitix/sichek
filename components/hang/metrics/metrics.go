package metrics

import (
	"github.com/scitix/sichek/components/hang/collector"
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
	HangGauge := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"name", "indicate_name"})
	return &HangMetrics{
		HangGauge: HangGauge,
	}
}

func (m *HangMetrics) ExportMetrics(hangInfo *collector.DeviceIndicatorStates) {
	for uuid, curIndicatorStates := range hangInfo.Indicators {
		for indicateName, indicator := range curIndicatorStates.Indicators {
			m.HangGauge.SetMetric("duration", []string{uuid, indicateName}, float64(indicator.Duration))
		}
	}
}
