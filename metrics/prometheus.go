package metrics

import (
	"log"
	"net/http"
	"sync"

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
	AnnotationResGauge  *GaugeVecMetricExporter
}

func newHealthCheckResMetrics() *HealthCheckResMetrics {
	HealthCheckResGauge := NewGaugeVecMetricExporter(MetricPrefix, []string{"component_name", "level", "error_name"})
	AnnotationResGauge := NewGaugeVecMetricExporter(MetricPrefix, []string{"annotaion"})

	return &HealthCheckResMetrics{
		HealthCheckResGauge: HealthCheckResGauge,
		AnnotationResGauge:  AnnotationResGauge,
	}
}

var HealthCheckMetrics *HealthCheckResMetrics
var once sync.Once

func GetHealthCheckResMetrics() *HealthCheckResMetrics {
	once.Do(func() {
		HealthCheckMetrics = newHealthCheckResMetrics()
	})
	return HealthCheckMetrics
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
func (m *HealthCheckResMetrics) ExportAnnotationMetrics(annoStr string) {
	m.AnnotationResGauge.SetMetric("node_annotaion", []string{annoStr}, 1.0)
}

func InitPrometheus() {
	// start Prometheus HTTP
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":9091", nil); err != nil {
		log.Fatal(err)
	}

}
