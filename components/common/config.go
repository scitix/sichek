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
package common

import (
	"time"
)

// spec
// config items
// type Config struct {
// 	Version    string                     `json:"version" yaml:"version"`
// 	Components map[string]ComponentConfig `json:"components,omitempty" yaml:"components,omitempty"`
// 	NodeInfo   map[string]string          `json:"node_info,omitempty" yaml:"node_info,omitempty"`
// }

// func (config *Config) Yaml() ([]byte, error) {
// 	return yaml.Marshal(config)
// }

// func LoadConfigFromYaml(file string) (*Config, error) {
// 	data, err := os.ReadFile(file)
// 	if err != nil {
// 		return nil, err
// 	}
// 	config := new(Config)
// 	err = yaml.Unmarshal(data, config)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return config, nil
// }

type ComponentConfig interface {
	GetCheckerSpec() map[string]CheckerSpec
	GetQueryInterval() time.Duration
	Yaml() (string, error)
	LoadFromYaml(file string) error
}

type CheckerSpec interface {
	JSON() (string, error)
	Yaml() (string, error)
	LoadFromYaml(file string) error
}
