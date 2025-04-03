package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/memory/collector"
)

var (
	memoryGaugeMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_gauge_metrics",
		},
		[]string{"component_name", "metric_name"},
	)
)

func InitMemoryMetrics() {
	prometheus.MustRegister(memoryGaugeMetrics)
}

func ExportMemoryMetrics(metrics *collector.MemoryInfo) {
	memoryGaugeMetrics.WithLabelValues("memory", "mem_total").Set(float64(metrics.MemTotal))
	memoryGaugeMetrics.WithLabelValues("memory", "mem_used").Set(float64(metrics.MemUsed))
	memoryGaugeMetrics.WithLabelValues("memory", "mem_free").Set(float64(metrics.MemFree))
	memoryGaugeMetrics.WithLabelValues("memory", "mem_percent_used").Set(float64(metrics.MemPercentUsed))
	memoryGaugeMetrics.WithLabelValues("memory", "mem_anonymous_used").Set(float64(metrics.MemAnonymousUsed))
	memoryGaugeMetrics.WithLabelValues("memory", "pagecache_used").Set(float64(metrics.PageCacheUsed))
	memoryGaugeMetrics.WithLabelValues("memory", "mem_unevictable_used").Set(float64(metrics.MemUnevictableUsed))
	memoryGaugeMetrics.WithLabelValues("memory", "dirty_page_used").Set(float64(metrics.DirtyPageUsed))
}
