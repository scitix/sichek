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

package config

import "github.com/scitix/sichek/components/common"

// RdmaEnvUserConfig is the top-level user config for the rdmaenv component.
type RdmaEnvUserConfig struct {
	RdmaEnv *RdmaEnvConfig `json:"rdmaenv" yaml:"rdmaenv"`
}

// RdmaEnvConfig holds the runtime tunables for scraping the rdma-env-pre exporter.
// There are no credentials: the exporter's /metrics endpoint has no auth.
type RdmaEnvConfig struct {
	QueryInterval common.Duration `json:"query_interval" yaml:"query_interval"`
	// Endpoint is the rdma-env-pre exporter URL, e.g. http://127.0.0.1:19099/metrics.
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// Timeout bounds a single scrape.
	Timeout common.Duration `json:"timeout" yaml:"timeout"`
	// MetricPrefix is the allow-list prefix: only series whose name starts with it are
	// passed through (guards against an endpoint that mixes in unrelated metrics).
	MetricPrefix string `json:"metric_prefix" yaml:"metric_prefix"`
	// EnableMetrics toggles re-exporting the scraped series on sichek's own /metrics.
	EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`
}

func (c *RdmaEnvUserConfig) GetQueryInterval() common.Duration {
	if c.RdmaEnv == nil {
		return common.Duration{}
	}
	return c.RdmaEnv.QueryInterval
}

func (c *RdmaEnvUserConfig) SetQueryInterval(newInterval common.Duration) {
	if c.RdmaEnv == nil {
		c.RdmaEnv = &RdmaEnvConfig{}
	}
	c.RdmaEnv.QueryInterval = newInterval
}
