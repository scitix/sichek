package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	nvconfig "github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

// PciDevice represents the device configuration
type PcieTopoSpec struct {
	NumaConfig        []*NumaConfig `json:"numa_config"`
	PciSwitchesConfig []*PciSwitch  `json:"pci_switches"`
}

type PcieTopoSpecs struct {
	Specs map[string]*PcieTopoSpec `json:"pcie_topo"`
}

type NumaConfig struct {
	NodeID   uint64 `json:"node_id"`
	GPUCount int    `json:"gpu_count"`
	IBCount  int    `json:"ib_count"`
}

type PciSwitch struct {
	GPU   int `json:"gpu"`
	IB    int `json:"ib"`
	Count int `json:"count"`
}

type BDFItem struct {
	DeviceType string `json:"type"`
	BDF        string `json:"bdf"`
}

func LoadSpec(file string) (*PcieTopoSpec, error) {
	s := &PcieTopoSpecs{}
	// 1. Load spec from provided file
	if file != "" {
		err := s.tryLoadFromFile(file)
		if err == nil {
			return FilterSpec(s)
		} else {
			logrus.WithField("component", "pcie").Warnf("failed to load from YAML file %s: %v", file, err)
		}
	}
	// 2. try to Load default spec from production env if no file specified
	// e.g., /var/sichek/config/default_spec.yaml
	err := s.tryLoadFromDefault()
	if err == nil {
		return FilterSpec(s)
	} else {
		logrus.WithField("component", "pcie").Warnf("%v", err)
	}

	// 3. try to load default spec from default config directory
	// for production env, it checks the default config path (e.g., /var/sichek/config/xx-component).
	// for development env, it checks the default config path based on runtime.Caller  (e.g., /repo/component/xx-component/config).
	err = s.tryLoadFromDevConfig()
	if err == nil {
		return FilterSpec(s)
	} else {
		logrus.WithField("component", "pcie").Warnf("%v", err)
	}
	return nil, fmt.Errorf("failed to load pcie_topo spec from any source, please check the configuration")
}

func (s *PcieTopoSpecs) tryLoadFromFile(file string) error {
	if file == "" {
		return fmt.Errorf("file path is empty")
	}
	err := utils.LoadFromYaml(file, s)
	if err != nil {
		return fmt.Errorf("failed to parse YAML file %s: %v", file, err)
	}

	if s.Specs == nil {
		return fmt.Errorf("YAML file %s loaded but contains no hca section", file)
	}
	logrus.WithField("component", "HCA").Infof("loaded default spec")
	return nil
}

func (s *PcieTopoSpecs) tryLoadFromDefault() error {
	specs := &PcieTopoSpecs{}
	err := common.LoadSpecFromProductionPath(specs)
	if err != nil || specs.Specs == nil {
		return fmt.Errorf("%v", err)
	}
	if s.Specs == nil {
		s.Specs = make(map[string]*PcieTopoSpec)
	}

	for id, spec := range specs.Specs {
		if _, ok := s.Specs[id]; !ok {
			s.Specs[id] = spec
		}
	}
	logrus.WithField("component", "HCA").Infof("loaded default spec")
	return nil
}

func (s *PcieTopoSpecs) tryLoadFromDevConfig() error {
	defaultDevCfgDirPath, files, err := common.GetDevDefaultConfigFiles(consts.ComponentNamePCIE)
	if err == nil {
		for _, file := range files {
			if strings.HasSuffix(file.Name(), consts.DefaultSpecSuffix) {
				specs := &PcieTopoSpecs{}
				filePath := filepath.Join(defaultDevCfgDirPath, file.Name())
				err := utils.LoadFromYaml(filePath, specs)
				if err != nil || specs.Specs == nil {
					return fmt.Errorf("failed to load from YAML file %s: %v", filePath, err)
				}
				if s.Specs == nil {
					s.Specs = make(map[string]*PcieTopoSpec)
				}
				for hcaName, hcaSpec := range specs.Specs {
					if _, ok := s.Specs[hcaName]; !ok {
						s.Specs[hcaName] = hcaSpec
					}
				}
			}
		}
	}
	return err
}

func FilterSpec(s *PcieTopoSpecs) (*PcieTopoSpec, error) {
	if s == nil || s.Specs == nil {
		return nil, fmt.Errorf("NvidiaSpecs is nil or empty")
	}
	localDeviceID, err := nvconfig.GetDeviceID()
	if err != nil {
		return nil, err
	}
	if spec, ok := s.Specs[localDeviceID]; ok {
		return spec, nil
	}
	return nil, fmt.Errorf("no device spec found for deviceID: %s", localDeviceID)
}
