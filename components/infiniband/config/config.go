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

type InfinibandUserConfig struct {
	Infiniband *InfinibandConfig `json:"infiniband" yaml:"infiniband"`
}

// InfinibandConfig 实现ComponentsConfig 接口
type InfinibandConfig struct {
	Name            string        `json:"name" yaml:"name"`
	QueryInterval   time.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64         `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers []string      `json:"ignored_checkers" yaml:"ignored_checkers"`
}

func (c *InfinibandUserConfig) GetComponentName() string {
	if c.Infiniband.Name != "" {
		return c.Infiniband.Name
	}
	return consts.ComponentNameInfiniband
}

func (c *InfinibandUserConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	return nil
}

func (c *InfinibandUserConfig) GetQueryInterval() time.Duration {
	return c.Infiniband.QueryInterval
}

func (c *InfinibandUserConfig) LoadUserConfigFromYaml(file string) error {
	if file == "" {
		return common.DefaultComponentConfig(consts.ComponentNameInfiniband, c, consts.DefaultUserCfgName)
	}
	err := utils.LoadFromYaml(file, c)
	if err != nil || c.Infiniband == nil {
		return fmt.Errorf("failed to load infiniband config: %v", err)
	}
	return nil
}
