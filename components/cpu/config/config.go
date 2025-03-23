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

type CPUConfig struct {
	CPU struct {
		Name          string                     `json:"name" yaml:"name"`
		QueryInterval time.Duration              `json:"query_interval" yaml:"query_interval"`
		CacheSize     int64                      `json:"cache_size" yaml:"cache_size"`
		EventCheckers map[string]*CPUEventConfig `json:"event_checkers" yaml:"event_checkers"`
	} `json:"cpu"`
}

func (c *CPUConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.CPU.EventCheckers {
		key := "event_" + name
		commonCfgMap[key] = cfg
	}
	return commonCfgMap
}

func (c *CPUConfig) GetQueryInterval() time.Duration {
	return c.CPU.QueryInterval
}

func (c *CPUConfig) GetCacheSize() int64 {
	return c.CPU.CacheSize
}

func (c *CPUConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *CPUConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *CPUConfig) LoadFromYaml(file string) error {
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

type CPUEventConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	LogFile     string `json:"log_file" yaml:"log_file"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
	Suggestion  string `json:"suggestion" yaml:"suggestion"`
}

func (c *CPUEventConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *CPUEventConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *CPUEventConfig) LoadFromYaml(file string) error {
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

func DefaultConfig(ctx context.Context) (*CPUConfig, error) {
	var config CPUConfig
	defaultCfgPath := "/config.yaml"
	_, err := os.Stat("/var/sichek/cpu" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/cpu/config.yaml
		defaultCfgPath = "/var/sichek/cpu" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + "/config.yaml"
	}

	err = config.LoadFromYaml(defaultCfgPath)
	return &config, err
}
