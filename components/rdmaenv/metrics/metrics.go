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

// Package metrics re-exports the scraped rdma-env-pre series verbatim on sichek's own
// Prometheus registry. It deliberately does NOT use sichek's GaugeVecMetricExporter:
// that helper force-appends a "node" label, prefixes+sanitizes the metric name, and
// fixes one label-key set per exporter -- all of which would break byte-for-byte
// passthrough. Here each upstream metric name gets its own GaugeVec with its own label
// keys, the name is kept exactly, and no extra label is added.
package metrics

import (
	"errors"
	"sort"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scitix/sichek/components/rdmaenv/collector"
	"github.com/sirupsen/logrus"
)

type RdmaEnvMetrics struct {
	reg    prometheus.Registerer
	mu     sync.Mutex
	gauges map[string]*prometheus.GaugeVec
	keys   map[string][]string // metric name -> sorted label keys (fixed on first sight)
}

// NewRdmaEnvMetrics uses the global default registry, which sichek's promhttp serves.
func NewRdmaEnvMetrics() *RdmaEnvMetrics {
	return newRdmaEnvMetricsWith(prometheus.DefaultRegisterer)
}

func newRdmaEnvMetricsWith(reg prometheus.Registerer) *RdmaEnvMetrics {
	return &RdmaEnvMetrics{
		reg:    reg,
		gauges: make(map[string]*prometheus.GaugeVec),
		keys:   make(map[string][]string),
	}
}

// ExportMetrics rebuilds the whole passthrough set from the current scrape: it resets
// every known GaugeVec first (so a series that vanished upstream does not linger), then
// re-sets the current series. On an unavailable scrape it resets and exports nothing.
func (m *RdmaEnvMetrics) ExportMetrics(info *collector.Info) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, g := range m.gauges {
		g.Reset()
	}
	if info == nil || !info.Available {
		return
	}

	for name, series := range info.Series {
		for i := range series {
			s := &series[i]
			gauge, keys := m.gaugeFor(name, s.Labels)
			if gauge == nil {
				continue
			}
			vals := make([]string, len(keys))
			for j, k := range keys {
				vals[j] = s.Labels[k]
			}
			gauge.WithLabelValues(vals...).Set(s.Value)
		}
	}
}

// gaugeFor returns the GaugeVec for a metric name, registering it on first sight with a
// fixed (sorted) label-key set. A later series whose key set differs is rejected to
// avoid a WithLabelValues panic.
func (m *RdmaEnvMetrics) gaugeFor(name string, labels map[string]string) (*prometheus.GaugeVec, []string) {
	if g, ok := m.gauges[name]; ok {
		if !sameKeys(m.keys[name], labels) {
			logrus.WithField("component", "rdmaenv").Warnf("metric %s label-key set changed, skipping series", name)
			return nil, nil
		}
		return g, m.keys[name]
	}

	keys := sortedKeys(labels)
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name}, keys)
	if err := m.reg.Register(g); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if existing, ok := are.ExistingCollector.(*prometheus.GaugeVec); ok {
				m.gauges[name] = existing
				m.keys[name] = keys
				return existing, keys
			}
		}
		logrus.WithField("component", "rdmaenv").Warnf("register metric %s failed: %v", name, err)
		return nil, nil
	}
	m.gauges[name] = g
	m.keys[name] = keys
	return g, keys
}

func sortedKeys(labels map[string]string) []string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sameKeys reports whether labels has exactly the key set want.
func sameKeys(want []string, labels map[string]string) bool {
	if len(want) != len(labels) {
		return false
	}
	for _, k := range want {
		if _, ok := labels[k]; !ok {
			return false
		}
	}
	return true
}
