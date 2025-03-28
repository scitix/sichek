package hca

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type HCASpec struct {
	HCAs map[string]*collector.IBHardWareInfo `json:"hca_specs"`
}

func (s *HCASpec) LoadDefaultSpec() error {
	defaultCfgDirPath := filepath.Join(consts.DefaultPodCfgPath, consts.ComponentNameHCA)
	_, err := os.Stat(defaultCfgDirPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		// 获取当前文件的目录
		defaultCfgDirPath = filepath.Dir(curFile)
	}
	files, err := os.ReadDir(defaultCfgDirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}
	if s.HCAs == nil {
		s.HCAs = make(map[string]*collector.IBHardWareInfo)
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
