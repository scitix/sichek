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

type GpfsEventRules struct {
	Rules *GpfsEventRule `yaml:"gpfs" json:"gpfs"`
}

type GpfsSpec struct {
	EventCheckers 		map[string]*GPFSEventConfig `json:"event_checkers" yaml:"event_checkers"`
	XStorHealthCheckers []string					`json:"xstor_health_checkers" yaml:"xstor_health_checkers"`
}

type GPFSEventConfig struct {
	Name    string `json:"name" yaml:"name"`
	LogFile string `json:"log_file" yaml:"log_file"`
	Regexp  string `json:"regexp" yaml:"regexp"`
}

func LoadDefaultEventRules() (*GpfsEventRule, error) {
	eventRules := &GpfsEventRules{}
	err := common.LoadDefaultEventRules(eventRules, consts.ComponentNameGpfs)
	if err == nil && eventRules.Rules != nil {
		return eventRules.Rules, nil
	}
	return nil, fmt.Errorf("failed to load eventRules: %v", err)
}
