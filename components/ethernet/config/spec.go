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

type EthernetSpecConfig struct {
	EthernetSpec *EthernetSpec `yaml:"ethernet" json:"ethernet"`
}

type EthernetSpec struct {
	// SoftwareDependencies SoftwareDependencies `yaml:"software_dependencies" json:"software_dependencies"`
	HWSpec []HWSpec `yaml:"hw_spec" json:"hw_spec"`
}

type HWSpec struct {
	Model          string         `yaml:"model" json:"model"`
	Type           string         `yaml:"type" json:"type"`
	Specifications Specifications `yaml:"specifications" json:"specifications"`
}

type Specifications struct {
	PhyState  string `yaml:"phy_state" json:"phy_state"`
	PortSpeed string `yaml:"port_speed" json:"port_speed"`
}

func (c *EthernetSpecConfig) LoadSpecConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, c)
		if err != nil || c.EthernetSpec == nil {
			logrus.WithField("componet", "ethernet").Errorf("failed to load spec from YAML file %s: %v", file, err)
		} else {
			logrus.WithField("component", "ethernet").Infof("loaded spec from YAML file %s", file)
			return nil
		}
	}
	err := common.DefaultComponentConfig(consts.ComponentNameEthernet, c, consts.DefaultSpecCfgName)
	if err != nil || c.EthernetSpec == nil {
		return fmt.Errorf("failed to load default ethernet spec: %v", err)
	}
	return nil
}
