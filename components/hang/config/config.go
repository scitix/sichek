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
	"fmt"

	"github.com/scitix/sichek/components/common"
)

type HangUserConfig struct {
	Hang *HangConfig `json:"hang" yaml:"hang"`
}

type HangConfig struct {
	QueryInterval   common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64           `json:"cache_size" yaml:"cache_size"`
	EnableMetrics   bool            `json:"enable_metrics" yaml:"enable_metrics"`
	NVSMI           bool            `json:"nvsmi" yaml:"nvsmi"`
	Mock            bool            `json:"mock" yaml:"mock"`
	IgnoreNamespace []string        `json:"ignore_namespaces" yaml:"ignore_namespaces"`

	ProcessedIgnoreNamespace map[string]struct{}
}

func (c *HangUserConfig) GetQueryInterval() common.Duration {
	return c.Hang.QueryInterval
}

// SetQueryInterval Update the query interval in the config
func (c *HangUserConfig) SetQueryInterval(newInterval common.Duration) {
	c.Hang.QueryInterval = newInterval
}

func (c *HangUserConfig) LoadUserConfigFromYaml(file string) error {
	err := common.LoadComponentUserConfig(file, c)
	if err != nil || c.Hang == nil {
		return fmt.Errorf("failed to load default hang user config: %v", err)
	}
	c.Hang.ProcessedIgnoreNamespace = make(map[string]struct{})
	for _, nameSpace := range c.Hang.IgnoreNamespace {
		c.Hang.ProcessedIgnoreNamespace[nameSpace] = struct{}{}
	}
	return nil
}
