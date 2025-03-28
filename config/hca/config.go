package hca

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type HCASpec struct {
	HCAs map[string]*collector.IBHardWareInfo `json:"hca_specs"`
}

func (s *HCASpec) LoadDefaultSpec() error {
	defaultCfgDirPath, err := utils.GetDefaultConfigDirPath(consts.ComponentNameHCA)
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
			hcaSpec := &HCASpec{}
			filePath := filepath.Join(defaultCfgDirPath, file.Name())
			err := utils.LoadFromYaml(filePath, hcaSpec)
			if err != nil {
				return fmt.Errorf("failed to load from YAML file %s: %v", filePath, err)
			}
			for hcaName, hcaSpec := range hcaSpec.HCAs {
				if _, ok := s.HCAs[hcaName]; !ok {
					s.HCAs[hcaName] = hcaSpec
				}
			}
		}
	}
	return nil
}
