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

import (
	"github.com/scitix/sichek/components/common"
)

type InfinibandUserConfig struct {
	Infiniband *InfinibandConfig `json:"infiniband" yaml:"infiniband"`
}

// InfinibandConfig implements ComponentsConfig interface
type InfinibandConfig struct {
	QueryInterval   common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64           `json:"cache_size" yaml:"cache_size"`
	EnableMetrics   bool            `json:"enable_metrics" yaml:"enable_metrics"`
	IgnoredCheckers []string        `json:"ignored_checkers" yaml:"ignored_checkers"`
}

func (c *InfinibandUserConfig) GetQueryInterval() common.Duration {
	return c.Infiniband.QueryInterval
}

func (c *InfinibandUserConfig) SetQueryInterval(newInterval common.Duration) {
	c.Infiniband.QueryInterval = newInterval
}
