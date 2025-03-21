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
	"time"

	"github.com/scitix/sichek/components/common"
	commonCfg "github.com/scitix/sichek/config"
)

type HangConfig struct {
	Name           string                      `json:"name" yaml:"name"`
	QueryInterval  time.Duration               `json:"query_interval" yaml:"query_interval"`
	CacheSize      int64                       `json:"cache_size" yaml:"cache_size"`
	CheckerConfigs map[string]*HangErrorConfig `json:"checkers" yaml:"checkers"`
	NVSMI          bool                        `json:"nvsmi" yaml:"nvsmi"`
	Mock           bool                        `json:"mock" yaml:"mock"`
}

func (c *HangConfig) GetQueryInterval() time.Duration {
	return c.QueryInterval
}

func (c *HangConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.CheckerConfigs {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

type HangIndicate struct {
	Name      string `json:"name" yaml:"name"`
	Threshold int64  `json:"threshold" yaml:"threshold"`
	CompareFn string `json:"compare" yaml:"compare"`
}

type HangErrorConfig struct {
	Name          string                   `json:"name" yaml:"name"`
	Description   string                   `json:"description,omitempty" yaml:"description,omitempty"`
	HangThreshold int64                    `json:"hang_threshold" yaml:"hang_threshold"`
	Level         string                   `json:"level" yaml:"level"`
	HangIndicates map[string]*HangIndicate `json:"check_items" yaml:"check_items"`
}

func (c *HangErrorConfig) LoadFromYaml(file string) error {
	err := commonCfg.LoadFromYaml(file, c)
	return err
}
