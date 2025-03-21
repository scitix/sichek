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

type MemoryConfig struct {
	Name          string                        `json:"name" yaml:"name"`
	QueryInterval time.Duration                 `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64                         `json:"cache_size" yaml:"cache_size"`
	Checkers      map[string]*MemoryEventConfig `json:"checkers" yaml:"checkers"`
}

func (c *MemoryConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.Checkers {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *MemoryConfig) GetQueryInterval() time.Duration {
	return c.QueryInterval
}

type MemoryEventConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	LogFile     string `json:"log_file" yaml:"log_file"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
}

func (c *MemoryEventConfig) LoadFromYaml(file string) error {
	err := commonCfg.LoadFromYaml(file, c)
	return err
}
