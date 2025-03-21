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

type NCCLConfig struct {
	NCCL struct {
		Name           string                      `json:"name" yaml:"name"`
		DirPath        string                      `json:"log_dir" yaml:"log_dir"`
		QueryInterval  time.Duration               `json:"query_interval" yaml:"query_interval"`
		CacheSize      int64                       `json:"cache_size" yaml:"cache_size"`
		CheckerConfigs map[string]*NCCLErrorConfig `json:"checkers" yaml:"checkers"`
	} `json:"NCCL"`
}

func (c *NCCLConfig) GetQueryInterval() time.Duration {
	return c.NCCL.QueryInterval
}

func (c *NCCLConfig) GetCacheSize() int64 {
	return c.NCCL.CacheSize
}

func (c *NCCLConfig) GetNCCLErrorConfig() map[string]*NCCLErrorConfig {
	return c.NCCL.CheckerConfigs
}

func (c *NCCLConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	commonCfgMap := make(map[string]common.CheckerSpec)
	for name, cfg := range c.NCCL.CheckerConfigs {
		commonCfgMap[name] = cfg
	}
	return commonCfgMap
}

func (c *NCCLConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *NCCLConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *NCCLConfig) LoadFromYaml(file string) error {
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

type NCCLErrorConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
}

func (c *NCCLErrorConfig) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *NCCLErrorConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *NCCLErrorConfig) LoadFromYaml(file string) error {
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

func DefaultConfig(ctx context.Context) (*NCCLConfig, error) {
	var ncclConfig NCCLConfig
	defaultCfgPath := "/config.yaml"
	_, err := os.Stat("/var/sichek/nccl" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/nccl/config.yaml
		defaultCfgPath = "/var/sichek/nccl" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = ncclConfig.LoadFromYaml(defaultCfgPath)
	return &ncclConfig, err
}
