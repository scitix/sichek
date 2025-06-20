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
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type CompareType string

const (
	CompareLow  CompareType = "low"
	CompareHigh CompareType = "high"
)

type HangEventRules struct {
	Rules *HangEventRule `yaml:"hang" json:"hang"`
}

type HangEventRule struct {
	Name                       string                    `json:"name" yaml:"name"`
	Description                string                    `json:"description,omitempty" yaml:"description,omitempty"`
	DurationThreshold          common.Duration           `json:"duration_threshold" yaml:"duration_threshold"`
	Level                      string                    `json:"level" yaml:"level"`
	Indicators                 map[string]*HangIndicator `json:"check_items" yaml:"check_items"`
	IndicatorsByModel          []*IndicatorModelOverride `json:"check_items_by_model" yaml:"check_items_by_model"`
	AbnormalDetectedTimes      uint32                    `json:"abnormal_detected_times" yaml:"abnormal_detected_times"`
	QueryIntervalAfterAbnormal common.Duration           `json:"query_interval_after_abnormal" yaml:"query_interval_after_abnormal"`
}

type HangIndicator struct {
	Threshold   int64  `json:"threshold" yaml:"threshold"`
	CompareType string `json:"compare" yaml:"compare"`
}

type IndicatorModelOverride struct {
	Model    string                    `yaml:"model" json:"model"`
	Override map[string]*HangIndicator `yaml:"override" json:"override"`
}

func LoadDefaultEventRules() (*HangEventRule, error) {
	eventRules := &HangEventRules{}
	err := common.LoadDefaultEventRules(eventRules, consts.ComponentNameHang)
	if err != nil {
		return nil, err
	}
	deviceID, err := config.GetDeviceID()
	if err != nil {
		return nil, err
	}
	for _, m := range eventRules.Rules.IndicatorsByModel {
		if m.Model == deviceID {
			for k, override := range m.Override {
				clone := *override
				eventRules.Rules.Indicators[k] = &clone
			}
			break
		}
	}
	return eventRules.Rules, nil
}

func LoadEventRules(file string) (*HangEventRule, error) {
	eventRules := &HangEventRules{}
	err := utils.LoadFromYaml(file, eventRules)
	if err != nil {
		return nil, err
	}
	deviceID, err := config.GetDeviceID()
	if err != nil {
		return nil, err
	}
	for _, m := range eventRules.Rules.IndicatorsByModel {
		if m.Model == deviceID {
			for k, override := range m.Override {
				clone := *override
				eventRules.Rules.Indicators[k] = &clone
			}
			break
		}
	}
	return eventRules.Rules, nil
}
