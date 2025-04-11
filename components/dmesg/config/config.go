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

type DmesgUserConfig struct {
	Dmesg *DmesgConfig `json:"dmesg" yaml:"dmesg"`
}

type DmesgConfig struct {
	Name           string                       `json:"name" yaml:"name"`
	DmesgFileName  []string                     `json:"FileNmae" yaml:"FileNmae"`
	DmesgCmd       [][]string                   `json:"Cmd" yaml:"Cmd"`
	QueryInterval  time.Duration                `json:"query_interval" yaml:"query_interval"`
	CacheSize      int64                        `json:"cache_size" yaml:"cache_size"`
	CheckerConfigs map[string]*DmesgErrorConfig `json:"checkers" yaml:"checkers"`
}

func (c *DmesgUserConfig) GetQueryInterval() time.Duration {
	return c.Dmesg.QueryInterval
}

func (c *DmesgUserConfig) SetQueryInterval(newInterval time.Duration) {
	c.Dmesg.QueryInterval = newInterval
}

func (c *DmesgUserConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.Dmesg.CheckerConfigs {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *DmesgUserConfig) LoadUserConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, c)
		if err != nil || c.Dmesg == nil {
			return fmt.Errorf("failed to load dmesg config from YAML file %s: %v", file, err)
		}
	}
	err := common.DefaultComponentConfig(consts.ComponentNameDmesg, c, consts.DefaultUserCfgName)
	if err != nil || c.Dmesg == nil {
		return fmt.Errorf("failed to load default dmesg config: %v", err)
	}
	return nil
}

type DmesgErrorConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
}
