package topotest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
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
	SwitchBDF string     `json:"switch_id"` // BDF for the PCIe switch
	BdfList   []*BDFItem `json:"bdf_list"`  // List of BDFs connected to this PCIe switch
}

func (sw *PciSwitch) String() string {
	var builder strings.Builder
	for _, item := range sw.BdfList {
		builder.WriteString(fmt.Sprintf(" %s: BDF=%s ", item.DeviceType, item.BDF))
	}
	return builder.String()
}

type BDFItem struct {
	DeviceType string `json:"type"`
	BDF        string `json:"bdf"`
}

func (c *PcieTopoConfig) LoadConfig(file string) error {
	if file != "" {
		err := utils.LoadFromYaml(file, c)
		if err != nil || len(c.PcieTopo) == 0 {
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

	err = utils.LoadFromYaml(file, c)
	if err != nil {
		return fmt.Errorf("failed to load pci topo config from %s ,error : %v", file, err)
	}

	return nil
}
