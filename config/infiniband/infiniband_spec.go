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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/config/hca"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type InfinibandSpec struct {
	Clusters map[string]*InfinibandSpecItem `json:"infiniband_cluster_specs"`
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
		logrus.WithField("infiniband", "Spec").Warnf("no valid cluster specification found for cluster %s, using default spec", clustetName)
		return s.Clusters["default"], nil
	}
}

func (s *InfinibandSpec) LoadDefaultSpec() error {
	defaultCfgDirPath, err := utils.GetDefaultConfigDirPath(consts.ComponentNameInfiniband)
	if err != nil {
		return err
	}
	files, err := os.ReadDir(defaultCfgDirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}

	// 遍历文件并加载符合条件的 YAML 文件
	for _, file := range files {
		if strings.HasSuffix(file.Name(), consts.DefaultSpecCfgSuffix) {
			infinibandSpec := &InfinibandSpec{}
			filePath := filepath.Join(defaultCfgDirPath, file.Name())
			err := utils.LoadFromYaml(filePath, infinibandSpec)
			if err != nil {
				return fmt.Errorf("failed to load from YAML file %s: %v", filePath, err)
			}
			for clusterName, infinibandSpec := range infinibandSpec.Clusters {
				if _, ok := s.Clusters[clusterName]; !ok {
					s.Clusters[clusterName] = infinibandSpec
				}
			}
		}
	}
	return nil
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
