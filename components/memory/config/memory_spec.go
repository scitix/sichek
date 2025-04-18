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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type MemorySpecConfig struct {
	MemorySpec *MemorySpec `yaml:"memory" json:"memory"`
}

type MemorySpec struct {
	EventCheckers map[string]*MemoryEventConfig `json:"event_checkers" yaml:"event_checkers"`
}

type MemoryEventConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	LogFile     string `json:"log_file" yaml:"log_file"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
}

func (c *MemorySpecConfig) LoadSpecConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, c)
		if err != nil || c.MemorySpec == nil {
			return fmt.Errorf("failed to load mempory spec from YAML file %s: %v", file, err)
		}
	}
	err := common.DefaultComponentConfig(consts.ComponentNameMemory, c, consts.DefaultSpecCfgName)
	if err != nil || c.MemorySpec == nil {
		return fmt.Errorf("failed to load default mempory spec: %v", err)
	}
	return nil
}
