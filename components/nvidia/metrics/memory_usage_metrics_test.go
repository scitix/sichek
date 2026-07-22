/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package metrics

import (
	"testing"

	"github.com/scitix/sichek/components/nvidia/collector"
	common "github.com/scitix/sichek/metrics"

	"github.com/stretchr/testify/assert"
)

// TestMemoryUsageMetricNames locks the exact Prometheus metric names exported
// for GPU memory usage. These names are a contract with the monitoring team;
// changing them here means coordinating a dashboard/alert update.
func TestMemoryUsageMetricNames(t *testing.T) {
	exporter := common.NewGaugeVecMetricExporter(MetricPrefix, []string{"index"})
	mem := collector.MemoryUsageInfo{
		UsedBytes:  10 << 30, // 10 GiB
		FreeBytes:  70 << 30, // 70 GiB
		TotalBytes: 80 << 30, // 80 GiB
	}
	exporter.ExportStruct(mem, []string{"0"}, TagPrefix)

	for _, name := range []string{
		"sichek_nvidia_memory_used_bytes",
		"sichek_nvidia_memory_free_bytes",
		"sichek_nvidia_memory_total_bytes",
	} {
		_, exists := exporter.MetricsMap[name]
		assert.Truef(t, exists, "expected metric %q to be exported", name)
	}
}
