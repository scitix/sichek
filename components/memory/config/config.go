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

type MemoryUserConfig struct {
	Memory *MemoryConfig `json:"memory" yaml:"memory"`
}

type MemoryConfig struct {
	Name          string                        `json:"name" yaml:"name"`
	QueryInterval time.Duration                 `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64                         `json:"cache_size" yaml:"cache_size"`
	Checkers      map[string]*MemoryEventConfig `json:"checkers" yaml:"checkers"`
}

func (c *MemoryUserConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.Memory.Checkers {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *MemoryUserConfig) GetQueryInterval() time.Duration {
	return c.Memory.QueryInterval
}

func (c *MemoryUserConfig) SetQueryInterval(newInterval time.Duration) {
	c.Memory.QueryInterval = newInterval
}

func (c *MemoryUserConfig) LoadUserConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, c)
		if err != nil || c.Memory == nil {
			return fmt.Errorf("failed to load memory config from YAML file %s: %v", file, err)
		}
	}
	err := common.DefaultComponentConfig(consts.ComponentNameMemory, c, consts.DefaultUserCfgName)
	if err != nil || c.Memory == nil {
		return fmt.Errorf("failed to load default memory config: %v", err)
	}
	return nil
}

type MemoryEventConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	LogFile     string `json:"log_file" yaml:"log_file"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
}
