package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/hang/checker"
)

var (
	hangGaugeMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hang_gauge_metrics",
		},
		[]string{"component_name", "metric_name", "dev_name", "item_name"},
	)
)

func InitHangMetrics() {
	prometheus.MustRegister(hangGaugeMetrics)
}

func ExportHangMetrics(hangInfo *checker.HangInfo) {

	for indicateName, name2duration := range hangInfo.HangDuration {
		for name, duration := range name2duration {
			hangGaugeMetrics.WithLabelValues("hang", "duration", name, indicateName).Set(float64(duration))
			hangGaugeMetrics.WithLabelValues("hang", "threshold", name, indicateName).Set(float64(hangInfo.HangThreshold[indicateName]))

		}
	}
}
