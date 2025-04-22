package topotest

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/scitix/sichek/pkg/utils"
)

type PcieTopoConfig struct {
	PcieTopo map[string]*MachineConfig `json:"pcie_machine"`
}

// PciDevice represents the device configuration
type MachineConfig struct {
	NumaConfig        []*NumaConfig `json:"numa_config"`
	PciSwitchesConfig []*PciSwitch  `json:"pci_switches"`
}

// NodeConfig represents the NUMA node configuration
type NumaConfig struct {
	NodeID  uint64     `json:"node_id"`  // NUMA Node ID
	BdfList []*BDFItem `json:"bdf_list"` // List of BDFs associated with the NUMA node
}

// PciSwitch represents the PCI switch configuration
type PciSwitch struct {
	SwitchID string     `json:"switch_id"` // BDF for the PCIe switch
	BdfList  []*BDFItem `json:"bdf_list"`  // List of BDFs connected to this PCIe switch
}

type BDFItem struct {
	DeviceType string `json:"type"`
	BDF        string `json:"bdf"`
}

func (c *PcieTopoConfig) LoadConfig(file string) error {
	if file == "" {
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		// Get the directory of the current file
		nowDir := filepath.Dir(curFile)
		file = filepath.Join(nowDir, "default_spec.yaml")
	}
	err := utils.LoadFromYaml(file, c)
	if err != nil {
		return fmt.Errorf("failed to load pci topo config: %v", err)
	}

	return nil
}
