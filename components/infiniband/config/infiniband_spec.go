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
	"runtime"

	"github.com/scitix/sichek/components/infiniband/collector"
	commonCfg "github.com/scitix/sichek/config"
	"github.com/sirupsen/logrus"
)

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

func resolveSpecPath(relPath string) (string, error) {
	const fallbackBasePath = commonCfg.DefaultPodCfgPath + commonCfg.ComponentNameInfiniband
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
		if err := commonCfg.LoadFromYaml(absPath, &hcaSpec); err != nil {
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
		if err := commonCfg.LoadFromYaml(absPath, &clusterSpecs); err != nil {
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
