package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	nvutils "github.com/scitix/sichek/components/nvidia/utils"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

// PciDevice represents the device configuration
type PcieTopoSpecs struct {
	Specs map[string]*PcieTopoSpec `json:"pcie_topo" yaml:"pcie_topo"`
}

type PcieTopoSpec struct {
	NumaConfig        []*NumaConfig `json:"numa_config" yaml:"numa_config"`
	PciSwitchesConfig []*PciSwitch  `json:"pci_switches" yaml:"pci_switches"`
}

type NumaConfig struct {
	NodeID   uint64 `json:"node_id" yaml:"node_id"`
	GPUCount int    `json:"gpu_count" yaml:"gpu_count"`
	IBCount  int    `json:"ib_count" yaml:"ib_count"`
}

type PciSwitch struct {
	GPU   int `json:"gpu" yaml:"gpu"`
	IB    int `json:"ib" yaml:"ib"`
	Count int `json:"count" yaml:"count"`
}

type BDFItem struct {
	DeviceType string `json:"type" yaml:"type"`
	BDF        string `json:"bdf" yaml:"bdf"`
}

func LoadSpec(file string) (*PcieTopoSpec, error) {
	s := &PcieTopoSpecs{}
	// 1. Load spec from provided file
	if file != "" {
		err := s.tryLoadFromFile(file)
		if err == nil && s.Specs != nil {
			return FilterSpec(s)
		} else {
			logrus.WithField("component", "pcie").Warnf("%v", err)
		}
	}
	// 2. try to Load default spec from production env if no file specified
	// e.g., /var/sichek/config/default_spec.yaml
	err := s.tryLoadFromDefault()
	if err == nil && s.Specs != nil {
		spec, err := FilterSpec(s)
		if err == nil {
			return spec, nil
		}
		logrus.WithField("component", "pcie").Warnf("failed to filter specs from default production top spec")
	} else {
		logrus.WithField("component", "pcie").Warnf("%v", err)
	}

	// 3. try to load default spec from default config directory
	// for production env, it checks the default config path (e.g., /var/sichek/config/xx-component).
	// for development env, it checks the default config path based on runtime.Caller  (e.g., /repo/component/xx-component/config).
	err = s.tryLoadFromDevConfig()
	if err == nil && s.Specs != nil {
		return FilterSpec(s)
	} else {
		if err != nil {
			logrus.WithField("component", "pcie").Warnf("failed to load from default dev directory: %v", err)
		} else {
			logrus.WithField("component", "pcie").Warnf("default dev spec loaded but contains no pcie_topo section")
		}
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
		return fmt.Errorf("YAML file %s loaded but contains no pcie_topo section", file)
	}
	logrus.WithField("component", "pcie").Infof("loaded default spec")
	return nil
}

func (s *PcieTopoSpecs) tryLoadFromDefault() error {
	specs := &PcieTopoSpecs{}
	err := common.LoadSpecFromProductionPath(specs)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	if specs.Specs == nil {
		return fmt.Errorf("default production top spec loaded but contains no pcie_topo section")
	}
	if s.Specs == nil {
		s.Specs = make(map[string]*PcieTopoSpec)
	}

	for id, spec := range specs.Specs {
		if _, ok := s.Specs[id]; !ok {
			s.Specs[id] = spec
		}
	}
	logrus.WithField("component", "pcie").Infof("loaded default production top spec")
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
					// If the file is not found or does not contain pcie topo specs, log the error
					// and continue to the next file.
					logrus.WithField("component", "pcie").Warnf("failed to load pcie spec from YAML file %s: %v", filePath, err)
					continue
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
		return nil, fmt.Errorf("pcie topo spec is nil or empty")
	}
	localDeviceID, err := nvutils.GetDeviceID()
	if err != nil {
		return nil, err
	}
	if spec, ok := s.Specs[localDeviceID]; ok {
		return spec, nil
	}
	return nil, fmt.Errorf("no device spec found for deviceID: %s", localDeviceID)
}
