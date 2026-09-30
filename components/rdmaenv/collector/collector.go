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

package collector

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultEndpoint = "http://127.0.0.1:19099/metrics"
	defaultPrefix   = "rdma_env_pre_"
	defaultTimeout  = 10 * time.Second
)

// Collector scrapes the rdma-env-pre exporter and turns its /metrics into an Info.
type Collector struct {
	endpoint string
	timeout  time.Duration
	prefix   string
}

// NewCollector builds a Collector, filling in defaults for empty/zero fields.
func NewCollector(endpoint string, timeout time.Duration, prefix string) *Collector {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if prefix == "" {
		prefix = defaultPrefix
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Collector{endpoint: endpoint, timeout: timeout, prefix: prefix}
}

// Collect scrapes once and returns an Info. A scrape failure is NOT surfaced as a Go
// error: it is recorded in Info (Available=false, Error set) so the daemon does not
// treat a missing/absent exporter as a component fault. The error return is always nil.
func (c *Collector) Collect(ctx context.Context) (*Info, error) {
	info := &Info{
		Endpoint:  c.endpoint,
		ScrapedAt: time.Now(),
		Summary:   Summary{VerdictCounts: map[string]int{}},
	}

	body, err := scrape(ctx, c.endpoint, c.timeout)
	if err != nil {
		info.Available = false
		info.Error = err.Error()
		logrus.WithField("component", "rdmaenv").Warnf("scrape %s failed: %v", c.endpoint, err)
		return info, nil
	}

	byName := ParseMetrics(body, c.prefix)
	info.Available = true
	info.Series = byName
	info.Summary = BuildSummary(byName)
	return info, nil
}
