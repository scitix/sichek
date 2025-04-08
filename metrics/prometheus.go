package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

var (
	CheckerResultsCounterMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sichek_results_counter_metrics",
		},
		[]string{"component_name", "node_name", "error_name", "device"},
	)
)

func InitCheckerResultsMetrics() {
	prometheus.MustRegister(CheckerResultsCounterMetrics)
}

func ExportCheckerResultsMetrics(metrics *common.Result) {
	for _, checker := range metrics.Checkers {
		if checker.Status == consts.StatusAbnormal {
			CheckerResultsCounterMetrics.WithLabelValues(metrics.Item, metrics.Node, checker.ErrorName, checker.Device).Set(1.0)
		} else {
			CheckerResultsCounterMetrics.WithLabelValues(metrics.Item, metrics.Node, checker.ErrorName, checker.Device).Set(0.0)
		}
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
