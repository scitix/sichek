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
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/scitix/sichek/components/common"
	hcaConfig "github.com/scitix/sichek/components/hca/config"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/oss"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type InfinibandSpecs struct {
	Specs map[string]*InfinibandSpec `json:"infiniband" yaml:"infiniband"`
}

type InfinibandSpec struct {
	IBPFDevs       map[string]string             `json:"ib_devs" yaml:"ib_devs"`
	IBSoftWareInfo *collector.IBSoftWareInfo     `json:"sw_deps" yaml:"sw_deps"`
	PCIeACS        string                        `json:"pcie_acs" yaml:"pcie_acs"`
	HCAs           map[string]*hcaConfig.HCASpec `json:"hca_specs,omitempty" yaml:"hca_specs,omitempty"`
}

func LoadSpec(file string) (*InfinibandSpec, error) {
	s := &InfinibandSpecs{}
	// 1. Load spec from provided file
	if file != "" {
		err := s.tryLoadFromFile(file)
		if err == nil && s.Specs != nil {
			return FilterSpec(s, file)
		}
		logrus.WithField("component", "infiniband").Warnf("%v", err)
	}
	// 2. try to Load default spec from production env if no file specified
	// e.g., /var/sichek/config/default_spec.yaml
	err := s.tryLoadFromDefault()
	if err == nil && s.Specs != nil {
		spec, err := FilterSpec(s, file)
		if err == nil {
			return spec, nil
		}
		logrus.WithField("component", "infiniband").Warnf("failed to filter specs from default production top spec")
	} else {
		logrus.WithField("component", "infiniband").Warnf("%v", err)
	}

	// 3. try to load default spec from default config directory
	// for production env, it checks the default config path (e.g., /var/sichek/config/xx-component).
	// for development env, it checks the default config path based on runtime.Caller  (e.g., /repo/component/xx-component/config).
	err = s.tryLoadFromDevConfig()
	if err == nil && s.Specs != nil {
		return FilterSpec(s, file)
	} else {
		if err != nil {
			logrus.WithField("component", "infiniband").Warnf("%v", err)
		} else {
			logrus.WithField("component", "infiniband").Warnf("default spec loaded but contains no infiniband section")
		}
	}

	return nil, fmt.Errorf("failed to load infiniband spec from any source, please check the configuration")
}

func (s *InfinibandSpecs) tryLoadFromFile(file string) error {
	if file == "" {
		return fmt.Errorf("file path is empty")
	}
	err := utils.LoadFromYaml(file, s)
	if err != nil {
		return fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}
	if s.Specs == nil {
		return fmt.Errorf("YAML file %s loaded but contains no infiniband section", file)
	}
	logrus.WithField("component", "infiniband").Infof("loaded default spec")
	return nil
}

func (s *InfinibandSpecs) tryLoadFromDefault() error {
	specs := &InfinibandSpecs{}
	err := common.LoadSpecFromProductionPath(specs)
	if err != nil {
		return err
	}
	if specs.Specs == nil {
		return fmt.Errorf("default top spec loaded but contains no infiniband section")
	}
	if s.Specs == nil {
		s.Specs = make(map[string]*InfinibandSpec)
	}
	for clusterName, spec := range specs.Specs {
		if _, ok := s.Specs[clusterName]; !ok {
			s.Specs[clusterName] = spec
		}
	}
	logrus.WithField("component", "infiniband").Infof("loaded default production top spec")
	return nil
}

func (s *InfinibandSpecs) tryLoadFromDevConfig() error {
	defaultDevCfgDirPath, files, err := common.GetDevDefaultConfigFiles(consts.ComponentNameInfiniband)
	if err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), consts.DefaultSpecSuffix) {
				specs := &InfinibandSpecs{}
				filePath := filepath.Join(defaultDevCfgDirPath, file.Name())
				err := utils.LoadFromYaml(filePath, specs)
				if err != nil {
					return fmt.Errorf("failed to load from YAML file %s: %v", filePath, err)
				}
				if s.Specs == nil {
					s.Specs = make(map[string]*InfinibandSpec)
				}
				for clusterName, clusterSpec := range specs.Specs {
					if _, ok := s.Specs[clusterName]; !ok {
						s.Specs[clusterName] = clusterSpec
					}
				}
			}
		}
	}
	return err
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

// FilterSpec retrieves the InfiniBand specification for the current cluster.
// If no specific cluster specification is found, it falls back to the default specification from OSS.
// If no default specification is found, it returns an error.
// It also loads the HCA specifications based on the hardware available on the node.
// If the HCA specifications cannot be loaded, it logs an error and returns the error.
// If the specification is nil, it returns an error indicating that the specification file is missing.
func FilterSpec(specs *InfinibandSpecs, file string) (*InfinibandSpec, error) {
	var ibSpec *InfinibandSpec
	if specs != nil && specs.Specs != nil {
		clusterName := extractClusterName()
		if spec, ok := specs.Specs[clusterName]; ok {
			ibSpec = spec
		} else {
			// If no specific cluster specification is found, fall back to the default specification
			ossIbSpec := &InfinibandSpecs{}
			url := fmt.Sprintf("%s/%s/%s.yaml", consts.DefaultOssCfgPath, consts.ComponentNameInfiniband, clusterName)
			logrus.WithField("component", "InfiniBand").Infof("Loading spec from OSS for clusterName %s: %s", clusterName, url)
			// Attempt to load spec from OSS
			err := oss.LoadSpecFromURL(url, ossIbSpec)
			if err == nil && ossIbSpec.Specs != nil {
				if spec, ok := ossIbSpec.Specs[clusterName]; ok {
					ibSpec = spec
				} else {
					if _, ok := specs.Specs["default"]; !ok {
						return nil, fmt.Errorf("no default infiniband specification found for cluster %s", clusterName)
					} else {
						logrus.WithField("infiniband", "spec").
							Warnf("No specific InfiniBand specification found for cluster %s; falling back to default specification", clusterName)
						ibSpec = specs.Specs["default"]
					}
				}
			} else {
				if _, ok := specs.Specs["default"]; !ok {
					return nil, fmt.Errorf("no default infiniband specification found for cluster %s", clusterName)
				} else {
					logrus.WithField("infiniband", "spec").
						Warnf("No specific InfiniBand specification found for cluster %s; falling back to default specification", clusterName)
					ibSpec = specs.Specs["default"]
				}
			}
		}
		// Get the board IDs of the IB devices in the host
		devBoardIDMap, ibDevs, err := hcaConfig.GetIBPFBoardIDs()
		if err != nil {
			return nil, err
		}
		specKeys := make([]string, 0, len(ibSpec.IBPFDevs))
		for k := range ibSpec.IBPFDevs {
			specKeys = append(specKeys, k)
		}
		currKeys := make([]string, 0, len(devBoardIDMap))
		for k := range devBoardIDMap {
			currKeys = append(currKeys, k)
		}
		changed := TrimMapByList(devBoardIDMap, ibSpec.IBPFDevs)
		if changed {
			logrus.WithField("component", "infiniband").
				Warnf("IB devices in the spec [%v] are not consistent with the current hardware[%v], trimming the spec to match the current hardware", specKeys, currKeys)
		}

		// Load HCA specs from provided file and merge with default specs
		// This will load from the provided file, merge with built-in specs (provided file has higher priority),
		// and load missing specs from OSS for all board IDs on the host
		hcaSpecs, err := hcaConfig.LoadSpec(file)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("failed to load HCA spec: %v", err)
			return nil, err
		}

		// Initialize ibSpec.HCAs if it's nil
		if ibSpec.HCAs == nil {
			ibSpec.HCAs = make(map[string]*hcaConfig.HCASpec)
		}

		// Check each board ID and fill in missing specs from hcaSpecs
		for _, boardID := range ibDevs {
			spec, exists := ibSpec.HCAs[boardID]
			if !exists || spec == nil {
				// If not found in ibSpec.HCAs, get it from hcaSpecs
				if hcaSpec, ok := hcaSpecs.HcaSpec[boardID]; ok {
					ibSpec.HCAs[boardID] = hcaSpec
					logrus.WithField("component", "infiniband").
						Infof("loaded HCA spec for board ID %s from HCA configs", boardID)
				} else {
					logrus.WithField("component", "infiniband").
						Warnf("spec for board ID %s not found in HCA configs", boardID)
				}
			} else if spec.Hardware.BoardID != boardID {
				// If board ID doesn't match, try to get from hcaSpecs
				logrus.WithField("component", "infiniband").
					Warnf("spec for board ID %s does not match the hardware board ID %s, trying to load from HCA configs", boardID, spec.Hardware.BoardID)
				if hcaSpec, ok := hcaSpecs.HcaSpec[boardID]; ok {
					ibSpec.HCAs[boardID] = hcaSpec
					logrus.WithField("component", "infiniband").
						Infof("replaced HCA spec for board ID %s from HCA configs", boardID)
				} else {
					logrus.WithField("component", "infiniband").
						Warnf("spec for board ID %s not found in HCA configs", boardID)
				}
			} else {
				logrus.WithField("component", "infiniband").
					Infof("spec for board ID %s matches the hardware board ID %s", boardID, spec.Hardware.BoardID)
			}
		}
		return ibSpec, nil
	}
	return nil, fmt.Errorf("infiniband specification is nil, please check the spec file %s", file)
}

// TrimMapByList removes keys from the map `b` that are not present in the slice `a`.
func TrimMapByList(a map[string]string, b map[string]string) bool {
	if len(a) >= len(b) {
		return false
	}
	changed := false
	for key := range b {
		if _, ok := a[key]; !ok {
			delete(b, key)
		}
		changed = true
	}
	return changed
}
