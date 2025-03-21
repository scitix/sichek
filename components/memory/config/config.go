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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/scitix/sichek/components/common"

	"sigs.k8s.io/yaml"
)

type MemoryConfig struct {
	Memory struct {
		Name          string                        `json:"name" yaml:"name"`
		QueryInterval time.Duration                 `json:"query_interval" yaml:"query_interval"`
		CacheSize     int64                         `json:"cache_size" yaml:"cache_size"`
		Checkers      map[string]*MemoryEventConfig `json:"checkers" yaml:"checkers"`
	} `json:"memory"`
}

func (c *MemoryConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.Memory.Checkers {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *MemoryConfig) GetQueryInterval() time.Duration {
	return c.Memory.QueryInterval
}

func (c *MemoryConfig) GetCacheSize() int64 {
	return c.Memory.CacheSize
}

func (c *MemoryConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *MemoryConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *MemoryConfig) LoadFromYaml(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		return err
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

func (c *MemoryEventConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *MemoryEventConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *MemoryEventConfig) LoadFromYaml(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		return err
	}
	return nil
}

func DefaultConfig(ctx context.Context) (*MemoryConfig, error) {
	var MemoryConfig MemoryConfig
	defaultCfgPath := "/memory_events.yaml"
	_, err := os.Stat("/var/sichek/memory" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/memory/memory_events.yaml
		defaultCfgPath = "/var/sichek/memory" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = MemoryConfig.LoadFromYaml(defaultCfgPath)
	return &MemoryConfig, err
}
