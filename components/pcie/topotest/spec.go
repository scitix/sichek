package topotest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type PcieTopoSpec struct {
	PcieTopo map[string]*MachineConfig `json:"pcie_topo"`
}

// PciDevice represents the device configuration
type MachineConfig struct {
	NumaConfig        []*NumaConfig `json:"numa_config"`
	PciSwitchesConfig []*PciSwitch  `json:"pci_switches"`
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

func (s *PcieTopoSpec) LoadSpec(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, s)
		if err != nil || len(s.PcieTopo) == 0 {
			logrus.WithField("componet", "pcietopo").Errorf("failed to load pci topo config from %s ,error : %v", file, err)
		} else {
			return nil
		}
	}

	file = filepath.Join(consts.DefaultPodCfgPath, "pcie/topotest/", consts.DefaultSpecCfgName)
	_, err := os.Stat(file)
	if err != nil {
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		// Get the directory of the current file
		nowDir := filepath.Dir(curFile)
		file = filepath.Join(nowDir, consts.DefaultSpecCfgName)
	}
	err = utils.LoadFromYaml(file, s)
	if err != nil {
		return fmt.Errorf("failed to load pci topo config from %s ,error : %v", file, err)
	}

	return nil
}
