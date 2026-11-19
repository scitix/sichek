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
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// serverModelInfo exposes the node's server_model_id (catcher 机型ID) as an
// info-style gauge. Only one series per node exists at a time.
var serverModelInfo = func() *prometheus.GaugeVec {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: MetricPrefix + "_server_model_info",
		Help: "Server model ID detected by catcher; value is always 1.",
	}, []string{"server_model_id", "complete", "node"})
	prometheus.MustRegister(g)
	return g
}()

// SetServerModelInfo replaces the node's server model series.
func SetServerModelInfo(id string, complete bool) {
	node, err := os.Hostname()
	if err != nil {
		node = "unknown"
	}
	serverModelInfo.Reset()
	serverModelInfo.WithLabelValues(id, strconv.FormatBool(complete), node).Set(1)
}
