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

type DmesgConfig struct {
	Name           string                       `json:"name" yaml:"name"`
	DmesgFileName  []string                     `json:"FileNmae" yaml:"FileNmae"`
	DmesgCmd       [][]string                   `json:"Cmd" yaml:"Cmd"`
	QueryInterval  time.Duration                `json:"query_interval" yaml:"query_interval"`
	CacheSize      int64                        `json:"cache_size" yaml:"cache_size"`
	CheckerConfigs map[string]*DmesgErrorConfig `json:"checkers" yaml:"checkers"`
}

func (c *DmesgConfig) GetQueryInterval() time.Duration {
	return c.QueryInterval
}

func (c *DmesgConfig) GetCacheSize() int64 {
	return c.CacheSize
}

func (c *DmesgConfig) GetDmesgErrorConfig() map[string]*DmesgErrorConfig {
	return c.CheckerConfigs
}

func (c *DmesgConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.CheckerConfigs {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *DmesgConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *DmesgConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *DmesgConfig) LoadFromYaml(file string) error {
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

type DmesgErrorConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
}

func (c *DmesgErrorConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *DmesgErrorConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *DmesgErrorConfig) LoadFromYaml(file string) error {
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

func DefaultConfig(ctx context.Context) (*DmesgConfig, error) {
	var dmesgConfig DmesgConfig
	defaultCfgPath := "/config.yaml"
	_, err := os.Stat("/var/sichek/dmesg" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/dmesg/config.yaml
		defaultCfgPath = "/var/sichek/dmesg" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = dmesgConfig.LoadFromYaml(defaultCfgPath)
	return &dmesgConfig, err
}
