package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scitix/sichek/components/common"
)

var (
	CheckerResultsCounterMetrics = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sichek_results_counter_metrics",
		},
		[]string{"component_name", "node_name", "status", "level", "error_name", "device"},
	)
)

func InitCheckerResultsMetrics() {
	prometheus.MustRegister(CheckerResultsCounterMetrics)
}

func ExportCheckerResultsMetrics(metrics *common.Result) {
	for _, checker := range metrics.Checkers {
		CheckerResultsCounterMetrics.WithLabelValues(metrics.Item, metrics.Node, metrics.Status, metrics.Level, checker.ErrorName, checker.Device).Inc()
	}
}

func InitPrometheus() {
	InitCheckerResultsMetrics()
	// 启动 Prometheus HTTP 处理程序
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":9091", nil); err != nil {
		log.Fatal(err)
	}

}
