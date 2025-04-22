package metrics

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
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

func InitPrometheus(cfgFile string) {
	// Initialize the metrics config
	cfg := &MetricsUserConfig{}
	err := common.LoadComponentUserConfig(cfgFile, cfg)
	if err != nil || cfg.Metrics == nil {
		logrus.WithField("component", "metrics").Errorf("InitPrometheus load user config failed or cfg is nil: %v", err)
	}
	// start Prometheus HTTP
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":"+strconv.Itoa(cfg.Metrics.Port), nil); err != nil {
		logrus.WithField("component", "metrics").Errorf("failed to start Prometheus metrics server: %v", err)
		return
	}
	logrus.WithField("component", "metrics").Infof("Prometheus metrics server started on port %d", cfg.Metrics.Port)
}
