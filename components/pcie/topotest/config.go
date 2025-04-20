package topotest

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/scitix/sichek/pkg/utils"
)

type PciTopoConfig struct {
	DeviceConfig  map[string]*PciDeviceTopoConfig  `json:"pcie_device"`
	ClusterConfig map[string]*PciClusterTopoConfig `json:"pcie_cluster"`
}

// PciDevice represents the device configuration
type PciDeviceTopoConfig struct {
	NumaConfig  []*NumaConfig `json:"numa_config"`
	PciSwitches []*PciSwitch  `json:"pci_switches"`
}

// NodeConfig represents the NUMA node configuration
type NumaConfig struct {
	NodeID  int      `json:"node_id"`  // NUMA Node ID
	BdfList []string `json:"bdf_list"` // List of BDFs associated with the NUMA node
}

// PciSwitch represents the PCI switch configuration
type PciSwitch struct {
	SwitchID string   `json:"switch_id"` // BDF for the PCIe switch
	BdfList  []string `json:"bdf_list"`  // List of BDFs connected to this PCIe switch
}

type PciClusterTopoConfig struct {
	Devices []string `json:"devices"`
}

func (c *PciTopoConfig) LoadConfig(file string) error {
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
