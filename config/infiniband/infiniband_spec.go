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
package infiniband

import (
	"fmt"
	"os"
	"regexp"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/config/hca"
)

type InfinibandSpec struct {
	Clusters map[string]*InfinibandSpecItem `json:"infiniband"`
	// Other fileds like `nvidia` can be added here if needed
}

type InfinibandSpecItem struct {
	IBDevs         []string                             `json:"ib_devs"`
	NetDevs        []string                             `json:"net_devs"`
	IBSoftWareInfo *collector.IBSoftWareInfo            `json:"sw_deps"`
	PCIeACS        string                               `json:"pcie_acs"`
	HCAs           map[string]*collector.IBHardWareInfo `json:"hca_specs"`
}

func (s *InfinibandSpec) LoadHCASpec(hcaSpecs *hca.HCASpec) error {
	if len(s.Clusters) == 0 {
		return fmt.Errorf("no valid cluster specification found")
	}
	for clusterName, detail := range s.Clusters {
		for psid, hcaSpec := range detail.HCAs {
			if hcaSpec.BoardID == "" {
				if _, ok := hcaSpecs.HCAs[psid]; ok {
					s.Clusters[clusterName].HCAs[psid] = hcaSpecs.HCAs[psid]
				} else {
					return fmt.Errorf("hca %s in cluster %s is not found in hca_spec or cluster spec yaml file", psid, clusterName)
				}
			}
		}
	}
	return nil
}

func (s *InfinibandSpec) GetClusterInfinibandSpec() (*InfinibandSpecItem, error) {

	clustetName := extractClusterName()
	if _, ok := s.Clusters[clustetName]; ok {
		return s.Clusters[clustetName], nil
	} else {
		return nil, fmt.Errorf("no valid cluster specification found for cluster %s", clustetName)
	}
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
