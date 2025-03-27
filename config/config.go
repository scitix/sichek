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
	"sync"

	"github.com/scitix/sichek/config/cpu"
	"github.com/scitix/sichek/config/infiniband"
	"github.com/scitix/sichek/config/hca"
	"github.com/scitix/sichek/config/nvidia"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"sigs.k8s.io/yaml"
)

// User Config: define use or ignore which component

type ComponentConfig struct {
	componentBasicConfig *BasicComponentConfigs
	componentSpecConfig  *SpecComponentConfigs
}
type BasicComponentConfigs struct {
	cpuBasicConfig    *cpu.CPUConfig       `json:"cpu" yaml:"cpu"`
	nvidiaBasicConfig *nvidia.NvidiaConfig `json:"nvidia" yaml:"nvidia"`
	infinibandBasicConfig *infiniband.InfinibandConfig `json:"infiniband" yaml:"infiniband"`
}
type SpecComponentConfigs struct {
	nvidiaSpecConfig *nvidia.NvidiaSpec `json:"nvidia" yaml:"nvidia"`
	infinibandSpecConfig *infiniband.InfinibandSpec `json:"infiniband" yaml:"infiniband"`
	hcaSpecConfig *hca.HCASpec `json:"hca" yaml:"hca"`
}

var (
	instance *ComponentConfig
	once     sync.Once
)

func LoadComponentConfig(basicConfigPath, specConfigPath string) (*ComponentConfig, error) {
	var err error
	once.Do(func() {
		instance = &ComponentConfig{}
		instance.componentBasicConfig = &BasicComponentConfigs{}
		if err = utils.LoadFromYaml(basicConfigPath, instance.componentBasicConfig); err != nil {
			return
		}

		instance.componentSpecConfig = &SpecComponentConfigs{}
		if err = utils.LoadFromYaml(specConfigPath, instance.componentSpecConfig); err != nil {
			return
		}
	})
	return instance, err
}

func (c *ComponentConfig) GetComponentConfigByComponentName(componentName string, defaultBasicConfig interface{}, defaultSpecConfig interface{}) (interface{}, interface{}) {
	if defaultBasicConfig == nil {
		DefaultComponentConfig(componentName, defaultBasicConfig, consts.DefaultBasicCfgName)
	}
	if defaultSpecConfig == nil {
		DefaultComponentConfig(componentName, defaultSpecConfig, consts.DefaultSpecCfgName)
	}

	return defaultBasicConfig, defaultSpecConfig
}

func (c *ComponentConfig) GetConfigByComponentName(componentName string) (interface{}, interface{}) {
	switch componentName {
	case consts.ComponentNameCPU:
		return c.GetComponentConfigByComponentName(componentName, c.componentBasicConfig.cpuBasicConfig, nil)
	case consts.ComponentNameNvidia:
		return c.GetComponentConfigByComponentName(componentName, c.componentBasicConfig.nvidiaBasicConfig, c.componentSpecConfig.nvidiaSpecConfig)
	case consts.ComponentNameInfiniband:
		return c.GetComponentConfigByComponentName(componentName, c.componentBasicConfig.infinibandBasicConfig, c.componentSpecConfig.infinibandSpecConfig)
	case consts.ComponentNameHCA:
		return c.GetComponentConfigByComponentName(componentName, nil, c.componentSpecConfig.hcaSpecConfig)
	default:
		return nil, nil // 未找到配置
	}
}

func DefaultComponentConfig(component string, config interface{}, filename string) error {
	defaultCfgPath := filepath.Join(consts.DefaultPodCfgPath, component, filename)
	_, err := os.Stat(defaultCfgPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		// 获取当前文件的目录
		defaultCfgPath = filepath.Join(filepath.Dir(curFile), component, filename)
	}
	err = utils.LoadFromYaml(defaultCfgPath, config)
	return err
}

type Config struct {
	Components map[string]bool `json:"components,omitempty" yaml:"components,omitempty"`
}

func (config *Config) Yaml() ([]byte, error) {
	return yaml.Marshal(config)
}

func LoadConfigFromYaml(file string) (*Config, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	config := new(Config)
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func GetDefaultConfig(useComponents []string, ignoreComponents []string) (*Config, error) {
	enabled_components := make(map[string]bool)
	if len(useComponents) == 0 {
		useComponents = consts.DefaultComponents
	}
	for _, component_name := range useComponents {
		enabled_components[component_name] = true
	}
	for _, component_name := range ignoreComponents {
		if _, exist := enabled_components[component_name]; exist {
			enabled_components[component_name] = false
		}
	}

	return &Config{
		Components: enabled_components,
	}, nil
}
