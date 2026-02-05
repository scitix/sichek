package metrics

import (
	"net"
	"net/http"
	"os"
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

func InitPrometheus(cfgFile string, metricsPort int, metricsSocket string) {
	// Priority: CLI metrics-socket > CLI metrics-port > config socket > config port > default port
	var port int
	var socket string

	cfg := &MetricsUserConfig{}
	if err := common.LoadUserConfig(cfgFile, cfg); err != nil || cfg.Metrics == nil {
		port = 19091
		logrus.WithField("component", "metrics").Debugf("InitPrometheus load user config failed or cfg is nil: %v", err)
	} else {
		port = cfg.Metrics.Port
		socket = cfg.Metrics.Socket
	}
	if metricsSocket != "" {
		socket = metricsSocket
		logrus.WithField("component", "metrics").Infof("Using metrics socket from command line: %s", socket)
	} else if metricsPort > 0 {
		port = metricsPort
		socket = ""
		logrus.WithField("component", "metrics").Infof("Using metrics port from command line: %d", port)
	}

	if socket == "" && port <= 0 {
		port = 19091
		logrus.WithField("component", "metrics").Warnf("Invalid or missing port, using default port %d", port)
	}

	http.Handle("/metrics", promhttp.Handler())

	if socket != "" {
		if _, err := os.Stat(socket); err == nil {
			os.Remove(socket)
		}
		listener, err := net.Listen("unix", socket)
		if err != nil {
			logrus.WithField("component", "metrics").Errorf("failed to listen on metrics socket %s: %v", socket, err)
			os.Exit(1)
		}
		logrus.WithField("component", "metrics").Infof("Starting Prometheus metrics server on socket %s", socket)
		if err := http.Serve(listener, nil); err != nil {
			logrus.WithField("component", "metrics").Errorf("failed to serve Prometheus metrics on socket: %v", err)
			os.Exit(1)
		}
		return
	}

	logrus.WithField("component", "metrics").Infof("Starting Prometheus metrics server on port %d", port)
	if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
		logrus.WithField("component", "metrics").Errorf("failed to start Prometheus metrics server: %v", err)
		os.Exit(1)
	}
}
