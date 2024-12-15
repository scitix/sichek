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
	commonCfg "github.com/scitix/sichek/config"
)

var (
	NOIBFOUND          = "No IB device found"
	ChekIBOFED         = "ofed"
	ChekIBFW           = "ib_fw"
	ChekIBState        = "ib_state"
	ChekIBPhyState     = "ib_phy_tate"
	ChekIBPortSpeed    = "ib_port_speed"
	CheckPCIEACS       = "pcie_acs"
	CheckPCIEMRR       = "pcie_mrr"
	CheckPCIESpeed     = "pcie_speed"
	CheckPCIEWidth     = "pcie_width"
	CheckPCIETreeSpeed = "pcie_tree_speed"
	CheckPCIETreeWidth = "pcie_tree_width"
	CheckIBKmod        = "ib_kmod"
	CheckIBDevs        = "ib_devs"
	checkDes           = map[string]string{
		ChekIBOFED:         "check the ofed is in spec",
		ChekIBFW:           "check the fw is same ver in spec",
		ChekIBState:        "check the ib state",
		ChekIBPhyState:     "check the ib phy statue",
		ChekIBPortSpeed:    "check the ib port speed",
		CheckPCIEACS:       "check the pcie acs is disabled",
		CheckPCIEMRR:       "check the pcie mrr is right setting",
		CheckPCIESpeed:     "check the pcie speed is right setting",
		CheckPCIEWidth:     "check the pcie width is right setting",
		CheckPCIETreeSpeed: "check the pcie tree speed is right setting",
		CheckPCIETreeWidth: "check the pcie tree width is right setting",
		CheckIBKmod:        "check the kernel module of ib",
		CheckIBDevs:        "check the ib dev list",
	}
	checkLevel = map[string]string{
		ChekIBOFED:         commonCfg.LevelWarning,
		ChekIBFW:           commonCfg.LevelWarning,
		ChekIBState:        commonCfg.LevelCritical,
		ChekIBPhyState:     commonCfg.LevelCritical,
		ChekIBPortSpeed:    commonCfg.LevelCritical,
		CheckPCIEACS:       commonCfg.LevelCritical,
		CheckPCIEMRR:       commonCfg.LevelCritical,
		CheckPCIESpeed:     commonCfg.LevelCritical,
		CheckPCIEWidth:     commonCfg.LevelCritical,
		CheckPCIETreeSpeed: commonCfg.LevelCritical,
		CheckPCIETreeWidth: commonCfg.LevelCritical,
		CheckIBKmod:        commonCfg.LevelCritical,
		CheckIBDevs:        commonCfg.LevelWarning,
	}
	errName = map[string]string{
		ChekIBOFED:         "the ofed is not in spec",
		ChekIBFW:           "ib fw version is not same",
		ChekIBState:        "ib status is down",
		ChekIBPhyState:     "ib phy link is down",
		ChekIBPortSpeed:    "ib port speed is not in spec",
		CheckPCIEACS:       "node pcie acs is not disable",
		CheckPCIEMRR:       "pcie mrr is not set to 4096",
		CheckPCIESpeed:     "pcie speed is not right",
		CheckPCIEWidth:     "pcie width is not right",
		CheckPCIETreeSpeed: "pcie speed is not right",
		CheckPCIETreeWidth: "pcie width is not right",
		CheckIBKmod:        "ib kernel module is not installed completely",
	}
	checkAction = map[string]string{
		ChekIBOFED:         "update the ofed",
		ChekIBFW:           "update the fw version",
		ChekIBState:        "ib state is not active, check opensm status",
		ChekIBPhyState:     "ib link is not link up, check link status",
		ChekIBPortSpeed:    "ib port speed is not set max",
		CheckPCIEACS:       "pcie acs is not disable, need to disable",
		CheckPCIEMRR:       "pcie mrr is not set right, set mrr to 4096",
		CheckPCIESpeed:     "need check the ibcard pcie speed",
		CheckPCIEWidth:     "need check the ibcard pcie width",
		CheckPCIETreeSpeed: "need check the ibcard pcie tree speed",
		CheckPCIETreeWidth: "need check the ibcard pcie Width speed",
		CheckIBKmod:        "need check the kernel module is all installed",
		CheckIBDevs:        "check the ib dev list",
	}
	checkDetail = map[string]string{
		ChekIBOFED:         "the ofed is right version",
		ChekIBFW:           "all ib use the same fw version include in spec",
		ChekIBState:        "all ib state is active",
		ChekIBPhyState:     "all ib phy link status is up",
		ChekIBPortSpeed:    "all ib port speed is right in spec",
		CheckPCIEACS:       "system all pcie acs is disabled",
		CheckPCIEMRR:       "ib mrr is set right(4096)",
		CheckPCIESpeed:     "ib pcie speed is right",
		CheckPCIEWidth:     "ib pcie width is right",
		CheckPCIETreeSpeed: "ib pcie tree speed is right",
		CheckPCIETreeWidth: "ib pcie tree width is right",
		CheckIBKmod:        "ib kernel module is all installed",
	}
)

var InfinibandCheckItems = map[string]common.CheckerResult{
	ChekIBOFED: {
		Name:        ChekIBOFED,
		Description: checkDes[ChekIBOFED],
		Status:      "",
		Level:       checkLevel[ChekIBOFED],
		Detail:      checkDetail[ChekIBOFED],
		ErrorName:   errName[ChekIBOFED],
		Suggestion:  checkAction[ChekIBOFED],
	},
	ChekIBFW: {
		Name:        ChekIBFW,
		Description: checkDes[ChekIBFW],
		Status:      "",
		Level:       checkLevel[ChekIBFW],
		Detail:      checkDetail[ChekIBFW],
		ErrorName:   errName[ChekIBFW],
		Suggestion:  checkAction[ChekIBFW],
	},
	ChekIBState: {
		Name:        ChekIBState,
		Description: checkDes[ChekIBState],
		Status:      "",
		Level:       checkLevel[ChekIBState],
		Detail:      checkDetail[ChekIBState],
		ErrorName:   errName[ChekIBState],
		Suggestion:  checkAction[ChekIBState],
	},
	ChekIBPhyState: {
		Name:        ChekIBPhyState,
		Description: checkDes[ChekIBPhyState],
		Status:      "",
		Level:       checkLevel[ChekIBPhyState],
		Detail:      checkDetail[ChekIBPhyState],
		ErrorName:   errName[ChekIBPhyState],
		Suggestion:  checkAction[ChekIBPhyState],
	},
	ChekIBPortSpeed: {
		Name:        ChekIBPortSpeed,
		Description: checkDes[ChekIBPortSpeed],
		Status:      "",
		Level:       checkLevel[ChekIBPortSpeed],
		Detail:      checkDetail[ChekIBPortSpeed],
		ErrorName:   errName[ChekIBPortSpeed],
		Suggestion:  checkAction[ChekIBPortSpeed],
	},
	CheckPCIEACS: {
		Name:        CheckPCIEACS,
		Description: checkDes[CheckPCIEACS],
		Status:      "",
		Level:       checkLevel[CheckPCIEACS],
		Detail:      checkDetail[CheckPCIEACS],
		ErrorName:   errName[CheckPCIEACS],
		Suggestion:  checkAction[CheckPCIEACS],
	},
	CheckPCIEMRR: {
		Name:        CheckPCIEMRR,
		Description: checkDes[CheckPCIEMRR],
		Status:      "",
		Level:       checkLevel[CheckPCIEMRR],
		Detail:      checkDetail[CheckPCIEMRR],
		ErrorName:   errName[CheckPCIEMRR],
		Suggestion:  checkAction[CheckPCIEMRR],
	},
	CheckPCIESpeed: {
		Name:        CheckPCIESpeed,
		Description: checkDes[CheckPCIESpeed],
		Status:      "",
		Level:       checkLevel[CheckPCIESpeed],
		Detail:      checkDetail[CheckPCIESpeed],
		ErrorName:   errName[CheckPCIESpeed],
		Suggestion:  checkAction[CheckPCIESpeed],
	},
	CheckPCIEWidth: {
		Name:        CheckPCIEWidth,
		Description: checkDes[CheckPCIEWidth],
		Status:      "",
		Level:       checkLevel[CheckPCIEWidth],
		Detail:      checkDetail[CheckPCIEWidth],
		ErrorName:   errName[CheckPCIEWidth],
		Suggestion:  checkAction[CheckPCIEWidth],
	},
	CheckPCIETreeSpeed: {
		Name:        CheckPCIETreeSpeed,
		Description: checkDes[CheckPCIETreeSpeed],
		Status:      "",
		Level:       checkLevel[CheckPCIETreeSpeed],
		Detail:      checkDetail[CheckPCIETreeSpeed],
		ErrorName:   errName[CheckPCIETreeSpeed],
		Suggestion:  checkAction[CheckPCIETreeSpeed],
	},
	CheckPCIETreeWidth: {
		Name:        CheckPCIETreeWidth,
		Description: checkDes[CheckPCIETreeWidth],
		Status:      "",
		Level:       checkLevel[CheckPCIETreeWidth],
		Detail:      checkDetail[CheckPCIETreeWidth],
		ErrorName:   errName[CheckPCIETreeWidth],
		Suggestion:  checkAction[CheckPCIETreeWidth],
	},
	CheckIBKmod: {
		Name:        CheckIBKmod,
		Description: checkDes[CheckIBKmod],
		Status:      "",
		Level:       checkLevel[CheckIBKmod],
		Detail:      checkDetail[CheckIBKmod],
		ErrorName:   errName[CheckIBKmod],
		Suggestion:  checkAction[CheckIBKmod],
	},
	CheckIBDevs: {
		Name:        CheckIBDevs,
		Description: checkDes[CheckIBDevs],
		Status:      "",
		Level:       checkLevel[CheckIBDevs],
		ErrorName:   errName[CheckIBDevs],
		Suggestion:  checkAction[CheckIBDevs],
	},
}