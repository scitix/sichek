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
	"github.com/scitix/sichek/components/common"
	nvutils "github.com/scitix/sichek/components/nvidia/utils"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

const (
	GPUHangCheckerName       = "GPUHang"
	SmClkStuckLowCheckerName = "SmClkStuckLow"
)

type CompareType string

const (
	CompareLow   CompareType = "low"
	CompareHigh  CompareType = "high"
	CompareEqual CompareType = "equal"
)

type GpuEventRules struct {
	Rules map[string]*GpuEventRule `yaml:"gpuevents" json:"gpuevents"`
}

type GpuEventRule struct {
	Name                       string                    `json:"name" yaml:"name"`
	Description                string                    `json:"description" yaml:"description"`
	DurationThreshold          common.Duration           `json:"duration_threshold" yaml:"duration_threshold"`
	Level                      string                    `json:"level" yaml:"level"`
	Indicators                 map[string]*HangIndicator `json:"check_items" yaml:"check_items"`
	IndicatorsByModel          []*IndicatorModelOverride `json:"check_items_by_model,omitempty" yaml:"check_items_by_model,omitempty"`
	AbnormalDetectedTimes      uint32                    `json:"abnormal_detected_times,omitempty" yaml:"abnormal_detected_times,omitempty"`
	QueryIntervalAfterAbnormal common.Duration           `json:"query_interval_after_abnormal,omitempty" yaml:"query_interval_after_abnormal,omitempty"`
}

type HangIndicator struct {
	Threshold   int64  `json:"threshold" yaml:"threshold"`
	CompareType string `json:"compare" yaml:"compare"`
}

type IndicatorModelOverride struct {
	Model    string                    `yaml:"model" json:"model"`
	Override map[string]*HangIndicator `yaml:"override" json:"override"`
}

func LoadDefaultEventRules() (map[string]*GpuEventRule, error) {
	eventRules := &GpuEventRules{}
	err := common.LoadDefaultEventRules(eventRules, consts.ComponentNameGpuEvents)
	if err != nil {
		return nil, err
	}
	deviceID, err := nvutils.GetDeviceID()
	if err != nil {
		return nil, err
	}
	for _, eventRule := range eventRules.Rules {
		for _, m := range eventRule.IndicatorsByModel {
			if m.Model == deviceID {
				for k, override := range m.Override {
					clone := *override
					eventRule.Indicators[k] = &clone
				}
				break
			}
		}
	}
	return eventRules.Rules, nil
}

func LoadEventRules(file string) (map[string]*GpuEventRule, error) {
	eventRules := &GpuEventRules{}
	err := utils.LoadFromYaml(file, eventRules)
	if err != nil {
		return nil, err
	}
	deviceID, err := nvutils.GetDeviceID()
	if err != nil {
		return nil, err
	}
	for _, eventRule := range eventRules.Rules {
		for _, m := range eventRule.IndicatorsByModel {
			if m.Model == deviceID {
				for k, override := range m.Override {
					clone := *override
					eventRule.Indicators[k] = &clone
				}
				break
			}
		}
	}
	return eventRules.Rules, nil
}
