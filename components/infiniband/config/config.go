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

	"sigs.k8s.io/yaml"
)

// 实现ComponentsConfig 接口
type InfinibandConfig struct {
	Name          string        `json:"name" yaml:"name"`
	QueryInterval time.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64         `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers      []string      `json:"ignored_checkers" yaml:"ignored_checkers"`
}

func (c *InfinibandConfig) ComponentName() string {
	return c.Name
}

func (c *InfinibandConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	return nil
}

func (c *InfinibandConfig) GetQueryInterval() time.Duration {
	return c.QueryInterval
}

func (c *InfinibandConfig) GetCacheSize() int64 {
	return c.CacheSize
}

func (c *InfinibandConfig) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *InfinibandConfig) LoadFromYaml(file string) error {
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

func DefaultConfig() (*InfinibandConfig, error) {
	var InfinbandConfig InfinibandConfig
	// 读取用户定义的检查项目
	defaultCfgPath := "/userDefaultChecker.yaml"
	_, err := os.Stat("/var/sichek/infiniband" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/infiniband/userDefaultChecker1.yaml
		defaultCfgPath = "/var/sichek/infiniband" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = InfinbandConfig.LoadFromYaml(defaultCfgPath)
	return &InfinbandConfig, err
}
