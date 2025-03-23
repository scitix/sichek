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

type HangConfig struct {
	Hang struct {
		Name           string                      `json:"name" yaml:"name"`
		QueryInterval  time.Duration               `json:"query_interval" yaml:"query_interval"`
		CacheSize      int64                       `json:"cache_size" yaml:"cache_size"`
		CheckerConfigs map[string]*HangErrorConfig `json:"checkers" yaml:"checkers"`
		NVSMI          bool                        `json:"nvsmi" yaml:"nvsmi"`
		Mock           bool                        `json:"mock" yaml:"mock"`
	} `json:"hang"`
}

func (c *HangConfig) GetQueryInterval() time.Duration {
	return c.Hang.QueryInterval
}

func (c *HangConfig) GetCacheSize() int64 {
	return c.Hang.CacheSize
}

func (c *HangConfig) GetHangErrorConfig() map[string]*HangErrorConfig {
	return c.Hang.CheckerConfigs
}

func (c *HangConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.Hang.CheckerConfigs {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *HangConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *HangConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *HangConfig) LoadFromYaml(file string) error {
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

type HangIndicate struct {
	Name      string `json:"name" yaml:"name"`
	Threshold int64  `json:"threshold" yaml:"threshold"`
	CompareFn string `json:"compare" yaml:"compare"`
}

type HangErrorConfig struct {
	Name          string                   `json:"name" yaml:"name"`
	Description   string                   `json:"description,omitempty" yaml:"description,omitempty"`
	HangThreshold int64                    `json:"hang_threshold" yaml:"hang_threshold"`
	Level         string                   `json:"level" yaml:"level"`
	HangIndicates map[string]*HangIndicate `json:"check_items" yaml:"check_items"`
}

func (c *HangErrorConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *HangErrorConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *HangErrorConfig) LoadFromYaml(file string) error {
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

func DefaultConfig(ctx context.Context) (*HangConfig, error) {
	var hangConfig HangConfig
	defaultCfgPath := "/config.yaml"
	_, err := os.Stat("/var/sichek/hang" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/hang/config.yaml
		defaultCfgPath = "/var/sichek/hang" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = hangConfig.LoadFromYaml(defaultCfgPath)
	return &hangConfig, err
}
