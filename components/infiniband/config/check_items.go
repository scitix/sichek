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
	CheckRoCE         = "check_roce"
	CheckIBDriver     = "check_ib_driver"

	CheckPCIEACS       = "check_pcie_acs"
	CheckPCIEMRR       = "check_pcie_mrr"
	CheckPCIETreeSpeed = "check_pcie_tree_speed"
	CheckPCIETreeWidth = "check_pcie_tree_width"
	CheckIBLost        = "check_ib_lost"
	CheckIBRailCount   = "check_ib_rail_count"
	CheckIBMezzName    = "check_ib_mezz_name"
)

// MezzBoardIDs is the set of firmware PSIDs (board_ids) that identify an internal
// IB mezzanine card. Per rdma-env-pre docs/mezz-card-identification.md board_id is
// the only reliable discriminator: mezz and CX7 both report PCI device 0x1021, so
// only board_id tells them apart. The board_id is generation-specific, so this set
// must be extended as new GPU generations ship. A mezz card has no HCA spec by
// design (it is identified but never configured).
var MezzBoardIDs = map[string]bool{
	"NVD0000000079": true, // B300
	"MT_0000001121": true, // B200
}

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
	CheckRoCE: {
		Name:        CheckRoCE,
		Description: "Check if RoCE vf is enabled",
		Level:       consts.LevelWarning,
		Detail:      "RoCE vf is enabled on all devices",
		ErrorName:   "RoCENotEnabled",
		Suggestion:  "Enable RoCE in the device configuration",
	},
	CheckIBLost: {
		Name:        CheckIBLost,
		Description: "Check if IB device is lost",
		Level:       consts.LevelCritical,
		Detail:      "No lost IB devices: all mlx5 PCIe functions healthy and HCA counts consistent",
		ErrorName:   "IBLost",
		Suggestion:  "Check IB device status",
	},
	CheckIBRailCount: {
		Name:        CheckIBRailCount,
		Description: "Check if the number of compute-rail HCAs is plausible (even, or a single rail)",
		Level:       consts.LevelCritical,
		Detail:      "Compute-rail HCA count is plausible",
		ErrorName:   "IBRailCountOdd",
		Suggestion:  "An odd rail count usually means an HCA vanished from the RDMA stack; compare against the node's expected topology and check dmesg for mlx5_core probe failures",
	},
	CheckIBMezzName: {
		Name:        CheckIBMezzName,
		Description: "Check that each mezz card (board_id NVD0000000079) RDMA device is named mezz_<k>",
		Level:       consts.LevelCritical,
		Detail:      "All mezz cards are named per the mezz_<k> convention",
		ErrorName:   "IBMezzNameMismatch",
		Suggestion:  "The mezz card was not renamed to mezz_<k>; ensure rdma-env-pre interface-naming ran on this node",
	},
}
