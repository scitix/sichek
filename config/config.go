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
	"os"

	"sigs.k8s.io/yaml"
)

// User Config: define use or ignore which component
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
		useComponents = DefaultComponents
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
