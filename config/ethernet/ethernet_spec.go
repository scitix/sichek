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
package ethernet

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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

func (c *EthernetSpec) LoadFromYaml(file string) error {
	// err := commonCfg.LoadFromYaml(file, c)
	return nil
}

func (c *EthernetSpec) GetEthSpec() (*EthernetSpec, error) {
	var ethernetConfig EthernetSpec
	defaultCfgPath := "/ethernetSpec.yaml"
	_, err := os.Stat("/var/sichek/ethernet" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/ethernet/ethernetSpec.yaml
		defaultCfgPath = "/var/sichek/ethernet" + defaultCfgPath
	} else {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = ethernetConfig.LoadFromYaml(defaultCfgPath)
	return &ethernetConfig, err
}
