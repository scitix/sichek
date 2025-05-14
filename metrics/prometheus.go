package metrics

import (
	"net/http"
	"reflect"
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

func getHealthCheckResMetricLables() []string {
	var checkerResult common.CheckerResult
	labelMap := make(map[string]*StructMetrics)
	StructToMetricsMap(reflect.ValueOf(checkerResult), "", "json", labelMap)
	labelNames := make([]string, 0, len(labelMap)+1)
	for k := range labelMap {
		if k != "detail" && k != "error_name" {
			labelNames = append(labelNames, k)
		}
	}
	return labelNames
}

func newHealthCheckResMetrics() *HealthCheckResMetrics {
	healthCheckResMetricLables := getHealthCheckResMetricLables()
	HealthCheckResGauge := NewGaugeVecMetricExporter(MetricPrefix, healthCheckResMetricLables)
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
		labelMap := make(map[string]*StructMetrics)
		StructToMetricsMap(reflect.ValueOf(checker), "", "json", labelMap)
		labelVals := make([]string, 0, len(m.HealthCheckResGauge.labelKeys)+1)
		for _, k := range m.HealthCheckResGauge.labelKeys {
			switch k {
			case "node":
				continue
			default:
				labelVals = append(labelVals, labelMap[k].StrLabel)
			}
		}
		metricName := metrics.Item + "_" + checker.ErrorName
		if checker.Status == consts.StatusAbnormal {
			m.HealthCheckResGauge.SetMetric(metricName, labelVals, 1.0)
		} else {
			m.HealthCheckResGauge.ResetMetric(metricName)
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
