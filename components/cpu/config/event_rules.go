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
)

type CpuEventRules struct {
	Rules *CpuEventRule `yaml:"cpu" json:"cpu"`
}

type CpuEventRule struct {
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

func LoadDefaultEventRules() (*CpuEventRule, error) {
	eventRules := &CpuEventRules{}
	err := common.LoadDefaultEventRules(eventRules, consts.ComponentNameCPU)
	if err == nil && eventRules.Rules != nil {
		return eventRules.Rules, nil
	}
	return nil, fmt.Errorf("failed to load eventRules: %v", err)
}
