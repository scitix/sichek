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
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

const (
	NOIBFOUND         = "no_ib_found"
	CheckIBOFED       = "check_ib_ofed"
	CheckIBNUM        = "check_ib_num"
	CheckIBFW         = "check_ib_fw"
	CheckIBState      = "check_ib_state"
	CheckIBPhyState   = "check_ib_phy_state"
	CheckNetOperstate = "check_net_operstate"
	CheckIBPortSpeed  = "check_ib_port_speed"
	CheckIBKmod       = "check_ib_kmod"
	CheckIBDevs       = "check_ib_devs"

	CheckPCIEACS       = "check_pcie_acs"
	CheckPCIEMRR       = "check_pcie_mrr"
	CheckPCIESpeed     = "check_pcie_speed"
	CheckPCIEWidth     = "check_pcie_width"
	CheckPCIETreeSpeed = "check_pcie_tree_speed"
	CheckPCIETreeWidth = "check_pcie_tree_width"
)

var InfinibandCheckItems = map[string]common.CheckerResult{
	CheckIBOFED: {
		Name:        CheckIBOFED,
		Description: "Check if the installed OFED version matches the specification",
		Level:       consts.LevelWarning,
		Detail:      "OFED version is within specification",
		ErrorName:   "OFEDVersionMismatch",
		Suggestion:  "Upgrade or reinstall OFED to match specification",
	},
	CheckIBNUM: {
		Name:        CheckIBNUM,
		Description: "Check if the number of IB devices matches PCI scan",
		Level:       consts.LevelCritical,
		Detail:      "All expected IB NICs are detected",
		ErrorName:   "IBDeviceCountMismatch",
		Suggestion:  "Check PCIe status or IB NIC connectivity",
	},
	CheckIBFW: {
		Name:        CheckIBFW,
		Description: "Check if firmware version matches the specification",
		Level:       consts.LevelWarning,
		Detail:      "Firmware version is consistent with spec",
		ErrorName:   "IBFirmwareVersionMismatch",
		Suggestion:  "Update firmware to match version in specification",
	},
	CheckIBState: {
		Name:        CheckIBState,
		Description: "Check if all IB ports are in ACTIVE state",
		Level:       consts.LevelCritical,
		Detail:      "All IB ports are in ACTIVE state",
		ErrorName:   "IBStateNotActive",
		Suggestion:  "Check OpenSM and IB connection",
	},
	CheckIBPhyState: {
		Name:        CheckIBPhyState,
		Description: "Check if all IB physical states are LINK_UP",
		Level:       consts.LevelCritical,
		Detail:      "All IB ports have LINK_UP physical state",
		ErrorName:   "IBPhyStateNotLinkUp",
		Suggestion:  "Verify IB cable and link status",
	},
	CheckNetOperstate: {
		Name:        CheckNetOperstate,
		Description: "Check if network operstate is UP",
		Level:       consts.LevelCritical,
		Detail:      "Network operstate is UP",
		ErrorName:   "IBNetOperStateNotUP",
		Suggestion:  "Check network interface and driver",
	},
	CheckIBPortSpeed: {
		Name:        CheckIBPortSpeed,
		Description: "Check if IB port speed is set to maximum",
		Level:       consts.LevelCritical,
		Detail:      "All IB ports run at maximum speed",
		ErrorName:   "IBPortSpeedNotMax",
		Suggestion:  "Ensure IB speed settings are correct in firmware",
	},
	CheckPCIEACS: {
		Name:        CheckPCIEACS,
		Description: "Check if PCIe ACS is disabled",
		Level:       consts.LevelCritical,
		Detail:      "PCIe ACS is disabled on all IB paths",
		ErrorName:   "PCIEACSNotDisabled",
		Suggestion:  "Disable ACS in BIOS or kernel settings",
	},
	CheckPCIEMRR: {
		Name:        CheckPCIEMRR,
		Description: "Check if PCIe Max Read Request (MRR) is set correctly (4096)",
		Level:       consts.LevelInfo,
		Detail:      "PCIe MRR is set correctly (4096)",
		ErrorName:   "PCIEMRRIncorrect",
		Suggestion:  "Set MRR to 4096 via system config",
	},
	CheckPCIESpeed: {
		Name:        CheckPCIESpeed,
		Description: "Check if PCIe link speed is optimal",
		Level:       consts.LevelCritical,
		Detail:      "PCIe speed matches device spec",
		ErrorName:   "PCIELinkSpeedDownDegraded",
		Suggestion:  "Ensure PCIe slot and firmware support correct speed",
	},
	CheckPCIEWidth: {
		Name:        CheckPCIEWidth,
		Description: "Check if PCIe link width is optimal",
		Level:       consts.LevelCritical,
		Detail:      "PCIe width matches device spec",
		ErrorName:   "PCIELinkWidthIncorrect",
		Suggestion:  "Verify PCIe lane configuration in BIOS",
	},
	CheckPCIETreeSpeed: {
		Name:        CheckPCIETreeSpeed,
		Description: "Check full PCIe tree speed to root complex",
		Level:       consts.LevelCritical,
		Detail:      "PCIe path to root complex supports full speed",
		ErrorName:   "PCIETreeSpeedDownDegraded",
		Suggestion:  "Check upstream PCIe device speed and configuration",
	},
	CheckPCIETreeWidth: {
		Name:        CheckPCIETreeWidth,
		Description: "Check full PCIe tree width to root complex",
		Level:       consts.LevelCritical,
		Detail:      "PCIe path to root complex supports full width",
		ErrorName:   "PCIETreeWidthIncorrect",
		Suggestion:  "Check PCIe switch and topology configuration",
	},
	CheckIBKmod: {
		Name:        CheckIBKmod,
		Description: "Check if all required IB kernel modules are installed",
		Level:       consts.LevelCritical,
		Detail:      "All IB kernel modules are loaded",
		ErrorName:   "IBKernelModulesNotAllInstalled",
		Suggestion:  "Install or reload missing kernel modules",
	},
	CheckIBDevs: {
		Name:        CheckIBDevs,
		Description: "Check if IB device names match expectation",
		Level:       consts.LevelWarning,
		Detail:      "IB device names are consistent",
		ErrorName:   "IBDeviceNameMismatch",
		Suggestion:  "Verify udev or naming rules",
	},
}
