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
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type CompareType string

const (
	CompareLow  CompareType = "low"
	CompareHigh CompareType = "high"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

type HangSpecConfig struct {
	HangSpec *HangSpec `yaml:"hang" json:"hang"`
}

type HangSpec struct {
	Name              string                    `json:"name" yaml:"name"`
	Description       string                    `json:"description,omitempty" yaml:"description,omitempty"`
	DurationThreshold Duration                  `json:"duration_threshold" yaml:"duration_threshold"`
	Level             string                    `json:"level" yaml:"level"`
	Indicators        map[string]*HangIndicator `json:"check_items" yaml:"check_items"`
	IndicatorsByModel []*IndicatorModelOverride `json:"check_items_by_model" yaml:"check_items_by_model"`
}

type HangIndicator struct {
	Threshold   int64  `json:"threshold" yaml:"threshold"`
	CompareType string `json:"compare" yaml:"compare"`
}

type IndicatorModelOverride struct {
	Model    string                    `yaml:"model" json:"model"`
	Override map[string]*HangIndicator `yaml:"override" json:"override"`
}

func (c *HangSpecConfig) GetSpec(specFile string) error {
	err := c.LoadSpecConfigFromYaml(specFile)
	if err != nil {
		return err
	}
	deviceID, err := config.GetDeviceID()
	if err != nil {
		return err
	}
	for _, m := range c.HangSpec.IndicatorsByModel {
		if m.Model == deviceID {
			for k, override := range m.Override {
				clone := *override
				c.HangSpec.Indicators[k] = &clone
			}
			break
		}
	}
	return nil
}

func (c *HangSpecConfig) LoadSpecConfigFromYaml(specFile string) error {
	if specFile != "" {
		err := utils.LoadFromYaml(specFile, c)
		if err != nil || c.HangSpec == nil {
			logrus.WithField("componet", "hang").Errorf("failed to load hang spec from YAML file %s: %v, try to load from default config", specFile, err)
		} else {
			logrus.WithField("component", "hang").Infof("loaded hang spec from YAML file %s", specFile)
			return nil
		}
	}
	err := common.DefaultComponentConfig(consts.ComponentNameHang, c, consts.DefaultSpecCfgName)
	if err != nil || c.HangSpec == nil {
		return fmt.Errorf("failed to load default hang spec: %v", err)
	}
	return nil
}
