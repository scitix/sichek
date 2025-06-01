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
	"github.com/sirupsen/logrus"
)

type CpuSpecConfig struct {
	CpuSpec *CpuSpec `yaml:"cpu" json:"cpu"`
}

type CpuSpec struct {
	EventCheckers map[string]*CPUEventConfig `json:"event_checkers" yaml:"event_checkers"`
}

type CPUEventConfig struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	LogFile     string `json:"log_file" yaml:"log_file"`
	Regexp      string `json:"regexp" yaml:"regexp"`
	Level       string `json:"level" yaml:"level"`
	Suggestion  string `json:"suggestion" yaml:"suggestion"`
}

func (c *CpuSpecConfig) LoadSpecConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, c)
		if err != nil || c.CpuSpec == nil {
			logrus.WithField("component", "cpu").Warnf("failed to load spec from YAML file %s: %v, try to load from default config", file, err)
		} else {
			logrus.WithField("component", "cpu").Infof("loaded spec from YAML file %s", file)
			return nil
		}
	}
	err := common.DefaultComponentConfig(consts.ComponentNameCPU, c, consts.DefaultSpecCfgName)
	if err != nil || c.CpuSpec == nil {
		return fmt.Errorf("failed to load default cpu spec: %v", err)
	}
	return nil
}
