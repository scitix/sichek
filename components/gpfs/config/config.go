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

type GpfsUserConfig struct {
	Gpfs *GpfsConfig `json:"gpfs" yaml:"gpfs"`
}

type GpfsConfig struct {
	Name          string                      `json:"name" yaml:"name"`
	QueryInterval time.Duration               `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64                       `json:"cache_size" yaml:"cache_size"`
	EventCheckers map[string]*GPFSEventConfig `json:"event_checkers" yaml:"event_checkers"`
}

func (c *GpfsUserConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.Gpfs.EventCheckers {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *GpfsUserConfig) GetQueryInterval() time.Duration {
	return c.Gpfs.QueryInterval
}

func (c *GpfsUserConfig) SetQueryInterval(newInterval time.Duration) {
	c.Gpfs.QueryInterval = newInterval
}

func (c *GpfsUserConfig) LoadUserConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, c)
		if err != nil || c.Gpfs == nil {
			return fmt.Errorf("failed to load gpfs config from YAML file %s: %v", file, err)
		}
	} else {
		err := common.DefaultComponentConfig(consts.ComponentNameGpfs, c, consts.DefaultUserCfgName)
		if err != nil || c.Gpfs == nil {
			return fmt.Errorf("failed to load default gpfs config: %v", err)
		}
	}
	return nil
}

type GPFSEventConfig struct {
	Name    string `json:"name" yaml:"name"`
	LogFile string `json:"log_file" yaml:"log_file"`
	Regexp  string `json:"regexp" yaml:"regexp"`
}
