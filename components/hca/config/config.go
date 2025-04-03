package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type HCASpecConfig struct {
	HcaSpec *HCASpec `json:"hca" yaml:"hca"`
}
type HCASpec struct {
	HCAHardwares map[string]*collector.IBHardWareInfo `json:"hardwares" yaml:"hardwares"`
}

func (s *HCASpecConfig) LoadSpecConfigFromYaml(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, s)
		if err != nil {
			logrus.WithField("component", "HCA").Errorf("failed to load from YAML file %s: %v", file, err)
		}
	}
	err := s.LoadDefaultSpec()
	if err != nil || s.HcaSpec == nil {
		return fmt.Errorf("failed to load default hca spec: %v", err)
	}
	return nil
}

func (s *HCASpecConfig) LoadDefaultSpec() error {
	if s.HcaSpec == nil {
		s.HcaSpec = &HCASpec{
			HCAHardwares: make(map[string]*collector.IBHardWareInfo),
		}
	}
	defaultCfgDirPath, files, err := common.GetDefaultConfigFiles(consts.ComponentNameHCA)
	if err != nil {
		return fmt.Errorf("failed to get default hca config files: %v", err)
	}
	// 遍历文件并加载符合条件的 YAML 文件
	for _, file := range files {
		if strings.HasSuffix(file.Name(), consts.DefaultSpecCfgSuffix) {
			hcaSpec := &HCASpecConfig{}
			filePath := filepath.Join(defaultCfgDirPath, file.Name())
			err := utils.LoadFromYaml(filePath, hcaSpec)
			if err != nil {
				return fmt.Errorf("failed to load from YAML file %s: %v", filePath, err)
			}
			for hcaName, hcaSpec := range hcaSpec.HcaSpec.HCAHardwares {
				if _, ok := s.HcaSpec.HCAHardwares[hcaName]; !ok {
					s.HcaSpec.HCAHardwares[hcaName] = hcaSpec
				}
			}
		}
	}
	return nil
}
