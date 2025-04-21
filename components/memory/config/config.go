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
	"github.com/sirupsen/logrus"
)

type MemoryUserConfig struct {
	Memory *MemoryConfig `json:"memory" yaml:"memory"`
}

type MemoryConfig struct {
	QueryInterval time.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64         `json:"cache_size" yaml:"cache_size"`
	EnableMetrics bool          `json:"enable_metrics" yaml:"enable_metrics"`
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
			logrus.WithField("component", "memory").Errorf("load user config from %s failed: %v, try to load from default config", file, err)
		} else {
			logrus.WithField("component", "memory").Infof("loaded user config from YAML file %s", file)
			return nil
		}
	}
	err := common.DefaultComponentUserConfig(c)
	if err != nil || c.Memory == nil {
		return fmt.Errorf("failed to load default memory user config: %v", err)
	}
	return nil
}
