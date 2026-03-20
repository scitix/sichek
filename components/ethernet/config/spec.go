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
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type EthernetSpecConfig struct {
	TargetBond     string `json:"target_bond" yaml:"target_bond"`
	BondMode       string `json:"bond_mode" yaml:"bond_mode"`
	MIIStatus      string `json:"mii_status" yaml:"mii_status"`
	LACPRate       string `json:"lacp_rate" yaml:"lacp_rate"`
	MTU            string `json:"mtu" yaml:"mtu"`
	Speed          string `json:"speed" yaml:"speed"`
	MinSlaves      int    `json:"min_slaves" yaml:"min_slaves"`
	XmitHashPolicy string `json:"xmit_hash_policy" yaml:"xmit_hash_policy"`
	Miimon         int    `json:"miimon" yaml:"miimon"`
	UpDelay        int    `json:"updelay" yaml:"updelay"`
	DownDelay      int    `json:"downdelay" yaml:"downdelay"`
}

type EthernetSpecs struct {
	Ethernet map[string]*EthernetSpecConfig `json:"ethernet" yaml:"ethernet"`
}

// LoadSpec loads Ethernet spec from the given file path.
func LoadSpec(file string) (*EthernetSpecConfig, error) {
	if file == "" {
		return nil, fmt.Errorf("ethernet spec file path is empty")
	}
	s := &EthernetSpecs{}
	if err := utils.LoadFromYaml(file, s); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}

	if s.Ethernet == nil {
		return nil, fmt.Errorf("ethernet spec is empty")
	}

	// For ethernet, we assume a "default" spec for now, similar to infiniband
	if spec, ok := s.Ethernet["default"]; ok {
		logrus.WithField("component", "ethernet").Infof("Loaded default Ethernet spec")
		return spec, nil
	}

	return nil, fmt.Errorf("default ethernet spec not found in provided specs")
}
