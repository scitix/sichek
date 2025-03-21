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

type NvidiaConfig struct {
	Spec            *NvidiaSpec
	ComponentConfig *ComponentConfig
}

func (c *NvidiaConfig) LoadFromYaml(userConfig string, specFile string) {
	c.Spec = GetSpec(specFile)
	c.ComponentConfig = &ComponentConfig{}
	err := commonCfg.LoadFromYaml(userConfig, c.ComponentConfig)
	if err != nil {
		c.ComponentConfig.SetDefault()
	}
}

type ComponentConfig struct {
	Nvidia struct {
		Name            string        `json:"name"`
		QueryInterval   time.Duration `json:"query_interval"`
		CacheSize       int64         `json:"cache_size"`
		IgnoredCheckers []string      `json:"ignored_checkers,omitempty"`
	} `json:"nvidia"`
}

func (c *ComponentConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	// for _, name := range c.IgnoredCheckers {
	// 	commonCfgMap[name] = {}
	// }
	return commonCfgMap
}
func (c *ComponentConfig) GetQueryInterval() time.Duration {
	return c.Nvidia.QueryInterval
}

func (c *ComponentConfig) SetDefault() {
	c.Nvidia.Name = "nvidia"
	c.Nvidia.QueryInterval = 10 * time.Second
	c.Nvidia.CacheSize = 10
	c.Nvidia.IgnoredCheckers = []string{}
}
