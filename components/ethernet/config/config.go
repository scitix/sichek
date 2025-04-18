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
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/pkg/utils"
)

type EthernetUserConfig struct {
	Ethernet *EthernetConfig `json:"ethernet" yaml:"ethernet"`
}

type EthernetConfig struct {
	Name            string        `json:"name" yaml:"name"`
	QueryInterval   time.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64         `json:"cache_size" yaml:"cache_size"`
	EnableMetrics   bool          `json:"enable_metrics" yaml:"enable_metrics"`
	IgnoredCheckers []string      `json:"ignored_checkers,omitempty"`
}

func (c *EthernetUserConfig) GetQueryInterval() time.Duration {
	return c.Ethernet.QueryInterval
}

func (c *EthernetUserConfig) SetQueryInterval(newInterval time.Duration) {
	c.Ethernet.QueryInterval = newInterval
}

func (c *EthernetUserConfig) LoadUserConfigFromYaml(file string) error {
	if file == "" {
		return common.DefaultComponentUserConfig(c)
	}
	err := utils.LoadFromYaml(file, c)
	if err != nil || c.Ethernet == nil {
		return fmt.Errorf("failed to load ethernet config: %v", err)
	}
	return nil
}
