/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0
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
	"os"
	"path/filepath"
	"regexp"
	"regexp"
	"runtime"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

/*
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
	Model          string         `yaml:"model" json:"model"` // Connectx_7
	Type           string         `yaml:"type" json:"type"`   // MT4129
	Specifications Specifications `yaml:"specifications" json:"specifications"`
}

type Specifications struct {
	Mode          []string            `yaml:"mode" json:"mode"`
	PortState     string              `yaml:"port_state" json:"port_state"`
	PortSpeed     []PortSpeedSpec     `yaml:"port_speed" json:"port_speed"`
	PhyState      string              `yaml:"phy_state" json:"phy_state"`
	PcieSpeed     []PcieSpeedSpec     `yaml:"pcie_speed" json:"pcie_speed"`
	PcieWidth     []PcieWidthSpec     `yaml:"pcie_width" json:"pcie_width"`
	PcieTreeSpeed []PcieTreeSpeedSpec `yaml:"pcie_tree_speed" json:"pcie_tree_speed"`
	PcieTreeWidth []PcieTreeWidthSpec `yaml:"pcie_tree_width" json:"pcie_tree_width"`
	PcieMrr       string              `yaml:"pcie_mrr" json:"pcie_mrr"`
	FWVer         []NicFWVer          `yaml:"fwVersion" json:"fwVersion"`
}

type NicFWVer struct {
	NodeName string `yaml:"node_name" json:"node_name"`
	FWverion string `yaml:"fw_version" json:"fw_version"`
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
	PCIESpeed string `yaml:"pcie_tree_speed" json:"pcie_tree_speed"`
}

type PcieTreeWidthSpec struct {
	NodeName  string `yaml:"node_name" json:"node_name"`
	PCIEWidth string `yaml:"pcie_tree_width" json:"pcie_tree_width"`
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

func GetClusterInfinibandSpec(specFile string) InfinibandSpec {
	infinibandSpec, err := GetInfinibandSpec(specFile)
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

	logrus.WithField("component", "infiniband").Infof("From Path: %v Get the IB Spec: %v", defaultCfgPath, InfinbandConfig)
	os.Exit(0)

	return &InfinbandConfig, err
}
*/

type ClusterSpec struct {
	Clusters map[string]InfinibandSpec `json:"infiniband"`
	// Other fileds like `nvidia` can be added here if needed
}

type HCASpec struct {
	HCAs map[string]collector.IBHardWareInfo `json:"hca_spec"`
}

type InfinibandSpec struct {
	IBDevs         []string                            `json:"ib_devs"`
	NetDevs        []string                            `json:"net_devs"`
	IBSoftWareInfo collector.IBSoftWareInfo            `json:"sw_deps"`
	PCIeACS        string                              `json:"pcie_acs"`
	HCAs           map[string]collector.IBHardWareInfo `json:"hca_specs"`
}

/*
// HCASpec 对应 `hca_spec` 的内容
type HCASpecDetail struct {
	HCAName string              `yaml:"hca_name" json:"hca_name"` // 注意字段名和标签的匹配
	OEM     map[string][]string `yaml:"oem" json:"oem"`
	SwDeps  SwDependencies      `yaml:"sw_deps" json:"sw_deps"` // 嵌套软件依赖
	HwSpec  HardwareSpec        `yaml:"hw_spec" json:"hw_spec"` // 嵌套硬件规格
}

// SwDependencies 软件依赖定义
type SwDependencies struct {
	Kmod      []string `yaml:"kmod" json:"kmod"`           // 列表类型
	Ofed      string   `yaml:"ofed" json:"ofed"`           // OFED 版本
	IBStatus  string   `yaml:"IBStatus" json:"IBStatus"`   // IB 状态
	Ethstatus string   `yaml:"Ethstatus" json:"Ethstatus"` // 以太网状态
	PcieAcs   string   `yaml:"pcie_acs" json:"pcie_acs"`   // PCIe ACS 设置
	PcieMrr   string   `yaml:"pcie_mrr" json:"pcie_mrr"`   // PCIe MRR 设置
}

// HardwareSpec 硬件规格定义
type HardwareSpec struct {
	HcaType       string `yaml:"hca_type" json:"hca_type"`               // HCA 类型
	HcaFw         string `yaml:"hca_fw" json:"hca_fw"`                   // HCA 固件版本
	NetPort       int    `yaml:"net_port" json:"net_port"`               // 端口数量 (整数类型)
	PortSpeed     string `yaml:"port_speed" json:"port_speed"`           // 端口速度
	PcieWidth     string `yaml:"pcie_width" json:"pcie_width"`           // PCIe 宽度
	PcieSpeed     string `yaml:"pcie_speed" json:"pcie_speed"`           // PCIe 速度
	PcieTreeWidth string `yaml:"pcie_tree_width" json:"pcie_tree_width"` // PCIe 树宽度
	PcieTreeSpeed string `yaml:"pcie_tree_speed" json:"pcie_tree_speed"` // PCIe 树速度
}
*/

func LoadFromYaml(file string, out interface{}) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, out)
}

func (c *ClusterSpec) JSON() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("JSON marshal failed: %w", err)
	}
	return string(data), nil
}

func (c *ClusterSpec) Yaml() (string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("YAML marshal failed: %w", err)
	}
	return string(data), nil
}

func resolveSpecPath(relPath string) (string, error) {
	const fallbackBasePath = "/var/sichek/infiniband"
	absPath := filepath.Join(fallbackBasePath, relPath)

	if _, err := os.Stat(absPath); err == nil {
		return absPath, nil
	}

	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get caller info")
	}
	return filepath.Join(filepath.Dir(curFile), relPath), nil
}

func GetHCASpec(specFile string) (*HCASpec, error) {
	log := logrus.WithField("component", "infiniband")

	var specFiles []string
	if specFile != "" {
		// Parse from the specified file
		specFiles = append(specFiles, specFile)
	} else {
		// Parse from the default files
		defaultSpecFiles := []string{
			"/default_ib_cx7_spec.yaml",
		}

		for _, relPath := range defaultSpecFiles {
			absPath, err := resolveSpecPath(relPath)
			if err != nil {
				log.Errorf("fail to resolve spec path: %v", err)
				return nil, fmt.Errorf("resolve path for %s failed: %w", relPath, err)
			}
			specFiles = append(specFiles, absPath)
		}
	}

	var hcaSpecs HCASpec
	hcaSpecs.HCAs = make(map[string]collector.IBHardWareInfo)
	for _, absPath := range specFiles {
		spec := ClusterSpec{}
		var hcaSpec HCASpec
		if err := LoadFromYaml(absPath, &hcaSpec); err != nil {
			log.Errorf("fail to load yaml: %s (err: %v)", absPath, err)
			continue
		}
		log.Infof("success load the hca spec: %s, spec:%v", absPath, spec)

		for devPSID, detail := range hcaSpec.HCAs {
			if _, exists := hcaSpecs.HCAs[devPSID]; !exists {
				hcaSpecs.HCAs[devPSID] = detail
			}
		}
	}

	if len(hcaSpecs.HCAs) == 0 {
		log.Warn("check the hca_spec yaml file, no valid format yaml found")
	}
	return &hcaSpecs, nil
}

func GetInfinibandSpec(specFile string) (*ClusterSpec, error) {
	log := logrus.WithField("component", "infiniband")

	hcaSpecs, _ := GetHCASpec("")

	var specFiles []string
	if specFile != "" {
		// Parse from the specified file
		specFiles = append(specFiles, specFile)
	} else {
		// Parse from the default files
		defaultSpecFiles := []string{
			"/default_infiniband_spec.yaml",
		}

		for _, relPath := range defaultSpecFiles {
			absPath, err := resolveSpecPath(relPath)
			if err != nil {
				return nil, fmt.Errorf("resolve path for %s failed: %w", relPath, err)
			}
			specFiles = append(specFiles, absPath)
		}
	}

	var infinibandSpec ClusterSpec
	infinibandSpec.Clusters = make(map[string]InfinibandSpec)
	for _, absPath := range specFiles {
		var clusterSpecs ClusterSpec
		if err := LoadFromYaml(absPath, &clusterSpecs); err != nil {
			log.Errorf("fail to load yaml: %s (err: %v)", absPath, err)
			continue
		}

		for clusterName, detail := range clusterSpecs.Clusters {
			if _, exists := infinibandSpec.Clusters[clusterName]; !exists {
				infinibandSpec.Clusters[clusterName] = detail
				for psid, hcaSpec := range detail.HCAs {
					if hcaSpec.BoardID == "" {
						if _, ok := hcaSpecs.HCAs[psid]; ok {
							infinibandSpec.Clusters[clusterName].HCAs[psid] = hcaSpecs.HCAs[psid]
						} else {
							log.Errorf("hca %s in cluster %s is not found in hca_spec or cluster spec yaml file", psid, clusterName)
						}
					}
				}
			}
		}
	}

	if len(infinibandSpec.Clusters) == 0 {
		return nil, fmt.Errorf("no valid cluster specification found")
	}
	return &infinibandSpec, nil
}

func extractClusterName() string {
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return "default"
	}
	re := regexp.MustCompile(`^([a-zA-Z]+)-?\d*`)
	matches := re.FindStringSubmatch(nodeName)
	if len(matches) > 1 {
		return matches[1]
	}
	return "default"
}

func GetClusterInfinibandSpec(specFile string) InfinibandSpec {
	infinibandSpec, err := GetInfinibandSpec(specFile)
	if err != nil {
		panic(err)
	}

	clustetName := extractClusterName()
	if _, ok := infinibandSpec.Clusters[clustetName]; ok {
		return infinibandSpec.Clusters[clustetName]
	} else {
		panic(fmt.Errorf("no valid cluster specification found for cluster %s", clustetName))
	}
}
