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

	hcaConfig "github.com/scitix/sichek/components/hca/config"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type InfinibandSpecs struct {
	Specs map[string]*InfinibandSpec `json:"infiniband" yaml:"infiniband"`
}

type InfinibandSpec struct {
	// as IBPFDevs may be trimmed on conditions, the HCANum will store the original number of HCA devices in spec
	HCANum         int                           `json:"hca_num,omitempty" yaml:"hca_num,omitempty"`
	IBPFDevs       map[string]string             `json:"ib_devs" yaml:"ib_devs"`
	IBSoftWareInfo *collector.IBSoftWareInfo     `json:"sw_deps" yaml:"sw_deps"`
	PCIeACS        string                        `json:"pcie_acs" yaml:"pcie_acs"`
	HCAs           map[string]*hcaConfig.HCASpec `json:"hca_specs,omitempty" yaml:"-"`
}

// LoadSpec loads infiniband spec from the given file path.
// The file path is expected to be already resolved by the command layer (e.g. via spec.EnsureSpecFile).
func LoadSpec(file string) (*InfinibandSpec, error) {
	if file == "" {
		return nil, fmt.Errorf("infiniband spec file path is empty")
	}
	s := &InfinibandSpecs{}
	if err := utils.LoadFromYaml(file, s); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}
	if s.Specs == nil {
		return nil, fmt.Errorf("YAML file %s loaded but contains no infiniband section", file)
	}
	logrus.WithField("component", "infiniband").Infof("loaded infiniband spec from %s", file)
	return FilterSpec(s, file)
}

// FilterSpec retrieves the InfiniBand specification for the current cluster.
// If no specific cluster specification is found, it falls back to the default specification from SICHEK_SPEC_URL.
// If no default specification is found, it returns an error.
// It also loads the HCA specifications based on the hardware available on the node.
// If the HCA specifications cannot be loaded, it logs an error and returns the error.
// If the specification is nil, it returns an error indicating that the specification file is missing.
func FilterSpec(specs *InfinibandSpecs, file string) (*InfinibandSpec, error) {
	var ibSpec *InfinibandSpec
	if specs != nil && specs.Specs != nil {
		clusterName := utils.ExtractClusterName()
		if spec, ok := specs.Specs[clusterName]; ok {
			logrus.WithField("infiniband", "spec").Warnf("using specific infiniband spec for cluster %s", clusterName)
			ibSpec = spec
		} else if spec, ok := specs.Specs["default"]; ok {
			logrus.WithField("infiniband", "spec").Infof("no spec for cluster %s, using default", clusterName)
			ibSpec = spec
		} else {
			return nil, fmt.Errorf("no infiniband specification for cluster %s and no default in %s", clusterName, file)
		}
		// Get the board IDs of the IB devices in the host
		devBoardIDMap, ibDevs, err := hcaConfig.GetIBPFBoardIDs()
		if err != nil {
			return nil, err
		}
		specKeys := make([]string, 0, len(ibSpec.IBPFDevs))
		ibSpec.HCANum = len(ibSpec.IBPFDevs)
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
		// and load missing specs from remote URL for all board IDs on the host
		hcaSpecs, err := hcaConfig.LoadSpec(file)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("failed to load HCA spec: %v", err)
			return nil, err
		}

		ibSpec.HCAs = make(map[string]*hcaConfig.HCASpec)

		// Check each board ID and fill in missing specs from hcaSpecs
		var missingBoardIDs []string
		for _, boardID := range ibDevs {
			if hcaSpec, ok := hcaSpecs.HcaSpec[boardID]; ok {
				ibSpec.HCAs[boardID] = hcaSpec
				logrus.WithField("component", "infiniband").
					Infof("loaded HCA spec for hardware board ID %s", boardID)
			} else {
				logrus.WithField("component", "infiniband").
					Warnf("spec for board ID %s not found", boardID)
				missingBoardIDs = append(missingBoardIDs, boardID)
			}
		}

		// Return error if any board IDs are missing specs
		if len(missingBoardIDs) > 0 {
			return ibSpec, fmt.Errorf("spec not found for board IDs: %v, please check the HCA configuration", missingBoardIDs)
		}

		return ibSpec, nil
	}
	return nil, fmt.Errorf("infiniband specification is nil, please check the spec file %s", file)
}

// TrimMapByList removes keys from the map `b` that are not present in the map `a`.
// Returns true if any key was removed from b.
func TrimMapByList(a map[string]string, b map[string]string) bool {
	changed := false
	for key := range b {
		if _, ok := a[key]; !ok {
			delete(b, key)
			changed = true
		}
	}
	return changed
}
