package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

const (
	MetricPrefix = "sichek"
	TagPrefix    = "json"
)

type HealthCheckResMetrics struct {
	HealthCheckResGauge *GaugeVecMetricExporter
}

func NewHealthCheckResMetrics() *HealthCheckResMetrics {
	HealthCheckResGauge := NewGaugeVecMetricExporter(MetricPrefix, []string{"component_name", "level", "error_name"})
	return &HealthCheckResMetrics{
		HealthCheckResGauge: HealthCheckResGauge,
	}
}

func (m *HealthCheckResMetrics) ExportMetrics(metrics *common.Result) {
	for _, checker := range metrics.Checkers {
		if checker.Status == consts.StatusAbnormal {
			m.HealthCheckResGauge.SetMetric("healthcheck_results", []string{metrics.Item, checker.Level, checker.ErrorName}, 1.0)
		} else {
			m.HealthCheckResGauge.SetMetric("healthcheck_results", []string{metrics.Item, checker.Level, checker.ErrorName}, 0.0)
		}
	}
}

func InitPrometheus() {
	// 启动 Prometheus HTTP 处理程序
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":9091", nil); err != nil {
		log.Fatal(err)
	}

}
