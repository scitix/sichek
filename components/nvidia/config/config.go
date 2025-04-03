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
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type NvidiaUserConfig struct {
	Nvidia *NvidiaConfig `json:"nvidia" yaml:"nvidia"`
}

type NvidiaConfig struct {
	Name            string        `json:"name"`
	QueryInterval   time.Duration `json:"query_interval"`
	CacheSize       int64         `json:"cache_size"`
	IgnoredCheckers []string      `json:"ignored_checkers,omitempty"`
}

func (c *NvidiaUserConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	// for _, name := range c.IgnoredCheckers {
	// 	commonCfgMap[name] = {}
	// }
	return commonCfgMap
}
func (c *NvidiaUserConfig) GetQueryInterval() time.Duration {
	return c.Nvidia.QueryInterval
}

func (c *NvidiaUserConfig) LoadUserConfigFromYaml(file string) error {
	if file == "" {
		return common.DefaultComponentConfig(consts.ComponentNameNvidia, c, consts.DefaultUserCfgName)
	}
	err := utils.LoadFromYaml(file, c)
	if err != nil || c.Nvidia == nil {
		return fmt.Errorf("failed to load nvidia config: %v", err)
	}
	return nil
}
