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

	"github.com/scitix/sichek/config/cpu_config"
	// "github.com/scitix/sichek/config/nvidia_config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/utils"
	"sigs.k8s.io/yaml"
)

// User Config: define use or ignore which component
type ComponentBasicConfig struct {
	cpuBasicConfig *cpu_config.CPUConfig `yaml:"cpu"`
	// nvidiaBasicConfig *nvidia_config.CPUConfig `yaml:"cpu"`
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

func DefaultConfig(component string, config interface{}) error {
	defaultCfgPath := consts.DefaultPodCfgPath + component + consts.DefaultCfgName
	_, err := os.Stat(defaultCfgPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		// 获取当前文件的目录
		currentDir := filepath.Dir(curFile)
		defaultCfgPath = filepath.Join(filepath.Dir(currentDir), "components", component, "config", consts.DefaultCfgName)
	}
	err = utils.LoadFromYaml(defaultCfgPath, config)
	return err
}
