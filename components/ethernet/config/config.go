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
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/scitix/sichek/components/common"

	"gopkg.in/yaml.v2"
)

type EthernetConfig struct {
	Ethernet struct {
	Name          string        `json:"name" yaml:"name"`
	QueryInterval time.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64         `json:"cache_size" yaml:"cache_size"`
	Cherkers      []string      `json:"checkers" yaml:"checkers"`
	}`json:"ethernet"`
}

func (c *EthernetConfig) ComponentName() string {
	return c.Ethernet.Name
}

func (c *EthernetConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	return nil
}

func (c *EthernetConfig) GetQueryInterval() time.Duration {
	return c.Ethernet.QueryInterval
}

func (c *EthernetConfig) GetCacheSize() int64 {
	return c.Ethernet.CacheSize
}

func (c *EthernetConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *EthernetConfig) LoadFromYaml(file string) error {
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

func DefaultConfig() (*EthernetConfig, error) {
	var ethernetConfig EthernetConfig
	defaultCfgPath := "/userDefaultChecker.yaml"
	_, err := os.Stat("/var/sichek/ethernet" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/ethernet/userDefaultChecker.yaml
		defaultCfgPath = "/var/sichek/ethernet" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = ethernetConfig.LoadFromYaml(defaultCfgPath)
	return &ethernetConfig, err
}
