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
	"github.com/scitix/sichek/components/hca/config"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type InfinibandSpecConfig struct {
	InfinibandSpec *InfinibandSpec `json:"infiniband" yaml:"infiniband"`
	// Other fileds like `nvidia` can be added here if needed
}

type InfinibandSpec struct {
	Clusters map[string]*InfinibandSpecItem `json:"clusters" yaml:"clusters"`
}

type InfinibandSpecItem struct {
	IBDevs         []string                             `json:"ib_devs"`
	NetDevs        []string                             `json:"net_devs"`
	IBSoftWareInfo *collector.IBSoftWareInfo            `json:"sw_deps"`
	PCIeACS        string                               `json:"pcie_acs"`
	HCAs           map[string]*collector.IBHardWareInfo `json:"hca_specs"`
}

func (s *InfinibandSpecConfig) LoadSpecConfigFromYaml(file string) error {
	hcaSpecs := &config.HCASpecConfig{}
	err := hcaSpecs.LoadSpecConfigFromYaml(file)
	if err != nil {
		return fmt.Errorf("failed to load hca spec from YAML file %s: %v", file, err)
	}
	if file != "" {
		err := utils.LoadFromYaml(file, s)
		if err != nil {
			return fmt.Errorf("failed to load IB spec from YAML file %s: %v", file, err)
		}
	}
	err = s.LoadDefaultSpec()
	if err != nil || s.InfinibandSpec == nil {
		return fmt.Errorf("failed to load default IB spec: %v", err)
	}
	err = s.LoadHCASpec(hcaSpecs)
	if err != nil {
		return fmt.Errorf("failed to load hca spec to IB spec: %v", err)
	}
	return nil
}

func (s *InfinibandSpecConfig) LoadHCASpec(hcaSpecs *config.HCASpecConfig) error {
	if len(s.InfinibandSpec.Clusters) == 0 {
		return fmt.Errorf("no valid cluster specification found")
	}
	for clusterName, detail := range s.InfinibandSpec.Clusters {
		for psid, hcaSpec := range detail.HCAs {
			if hcaSpec.BoardID == "" {
				if _, ok := hcaSpecs.HcaSpec.HCAHardwares[psid]; ok {
					s.InfinibandSpec.Clusters[clusterName].HCAs[psid] = hcaSpecs.HcaSpec.HCAHardwares[psid]
				} else {
					return fmt.Errorf("hca %s in cluster %s is not found in hca_spec or cluster spec yaml file", psid, clusterName)
				}
			}
		}
	}
	return nil
}

func (s *InfinibandSpecConfig) LoadDefaultSpec() error {
	if s.InfinibandSpec == nil {
		s.InfinibandSpec = &InfinibandSpec{
			Clusters: make(map[string]*InfinibandSpecItem),
		}
	}
	defaultCfgDirPath, files, err := common.GetDefaultConfigFiles(consts.ComponentNameInfiniband)
	if err != nil {
		return fmt.Errorf("failed to get default infiniband config files: %v", err)
	}
	// 遍历文件并加载符合条件的 YAML 文件
	for _, file := range files {
		if strings.HasSuffix(file.Name(), consts.DefaultSpecCfgSuffix) {
			infinibandSpec := &InfinibandSpecConfig{}
			filePath := filepath.Join(defaultCfgDirPath, file.Name())
			err := utils.LoadFromYaml(filePath, infinibandSpec)
			if err != nil {
				return fmt.Errorf("failed to load from YAML file %s: %v", filePath, err)
			}
			for clusterName, infinibandSpec := range infinibandSpec.InfinibandSpec.Clusters {
				if _, ok := s.InfinibandSpec.Clusters[clusterName]; !ok {
					s.InfinibandSpec.Clusters[clusterName] = infinibandSpec
				}
			}
		}
	}
	return nil
}

func (s *InfinibandSpecConfig) GetClusterInfinibandSpec() (*InfinibandSpecItem, error) {
	clustetName := extractClusterName()
	if _, ok := s.InfinibandSpec.Clusters[clustetName]; ok {
		return s.InfinibandSpec.Clusters[clustetName], nil
	} else {
		logrus.WithField("infiniband", "Spec").Warnf("no valid cluster specification found for cluster %s, using default spec", clustetName)
		return s.InfinibandSpec.Clusters["default"], nil
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
