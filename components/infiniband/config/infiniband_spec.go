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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"sigs.k8s.io/yaml"
)

type InfinibandHCASpec struct {
	SoftwareDependencies SoftwareDependencies `yaml:"software_dependencies" json:"software_dependencies"`
	IBDevs               []string             `yaml:"ib_devs" json:"ib_devs"`
	HWSpec               []HWSpec             `yaml:"hw_spec" json:"hw_spec"`
}

type SoftwareDependencies struct {
	OFED         []OFEDSpec `yaml:"ofed" json:"ofed"`
	KernelModule []string   `yaml:"kernel_module" json:"kernel_module"`
	Libraries    []string   `yaml:"libraries" json:"libraries"`
	Tools        []string   `yaml:"tools" json:"tools"`
	PcieACS      string     `yaml:"pcie_acs" json:"pcie_acs"`
}

type HWSpec struct {
	Model          string         `yaml:"model" json:"model"`
	Type           string         `yaml:"type" json:"type"`
	Specifications Specifications `yaml:"specifications" json:"specifications"`
}

type Specifications struct {
	PortSpeed     []PortSpeedSpec     `yaml:"port_speed" json:"port_speed"`
	Mode          []string            `yaml:"mode" json:"mode"`
	PortState     string              `yaml:"port_state" json:"port_state"`
	PhyState      string              `yaml:"phy_state" json:"phy_state"`
	PcieSpeed     []PcieSpeedSpec     `yaml:"pcie_speed" json:"pcie_speed"`
	PcieWidth     []PcieWidthSpec     `yaml:"pcie_width" json:"pcie_width"`
	PcieTreeSpeed []PcieTreeSpeedSpec `yaml:"pcie_tree_speed" json:"pcie_tree_speed"`
	PcieTreeWidth []PcieTreeWidthSpec `yaml:"pcie_tree_width" json:"pcie_tree_width"`
	PcieMrr       string              `yaml:"pcie_mrr" json:"pcie_mrr"`
	Ports         int                 `yaml:"ports" json:"ports"`
	FwVersion     string              `yaml:"fwVersion" json:"fwVersion"`
}

type PcieSpeedSpec struct {
	NodeName  string `yaml:"node_name" json:"node_name"`
	PCIESpeed string `yaml:"pcie_speed" json:"pcie_speed"`
}

type PcieWidthSpec struct {
	NodeName  string `yaml:"node_name" json:"node_name"`
	PCIEWidth string `yaml:"pcie_width" json:"pcie_width"`
}

type PcieTreeSpeedSpec struct {
	NodeName  string `yaml:"node_name" json:"node_name"`
	PCIESpeed string `yaml:"pcie_speed" json:"pcie_speed"`
	PCIEBDF   string `yaml:"pcie_bdf" json:"pcie_bdf"`
}

type PcieTreeWidthSpec struct {
	NodeName  string `yaml:"node_name" json:"node_name"`
	PCIEWidth string `yaml:"pcie_width" json:"pcie_width"`
}

type OFEDSpec struct {
	NodeName string `yaml:"node_name" json:"node_name"`
	OFEDVer  string `yaml:"ofed_ver" json:"ofed_ver"`
}

type PortSpeedSpec struct {
	NodeName string `yaml:"node_name" json:"node_name"`
	Speed    string `yaml:"speed" json:"speed"`
}

func (c *InfinibandHCASpec) JSON() (string, error) {
	data, err := json.Marshal(c)
	return string(data), err
}

func (c *InfinibandHCASpec) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	return string(data), err
}

func (c *InfinibandHCASpec) LoadFromYaml(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, c)
	if err != nil {
		return err
	}
	return nil
}

func (c *InfinibandHCASpec) GetHCASpec() (*InfinibandHCASpec, error) {
	var InfinbandConfig InfinibandHCASpec
	defaultCfgPath := "/infinibandSpec.yaml"
	_, err := os.Stat("/var/sichek/infiniband" + defaultCfgPath)
	if err == nil {
		// run in pod use /var/sichek/infiniband/infinibandSpec.yaml
		defaultCfgPath = "/var/sichek/infiniband" + defaultCfgPath
	} else {
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return nil, fmt.Errorf("get curr file path failed")
		}

		defaultCfgPath = filepath.Dir(curFile) + defaultCfgPath
	}

	err = InfinbandConfig.LoadFromYaml(defaultCfgPath)
	return &InfinbandConfig, err
}
