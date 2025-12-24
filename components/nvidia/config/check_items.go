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
	NVFabricManagerCheckerName           = "nvidia-fabricmanager"
	IOMMUCheckerName                     = "iommu"
	NvPeerMemCheckerName                 = "nvidia_peermem"
	PCIeACSCheckerName                   = "pcie-acs"
	SRAMAggUncorrectableCheckerName      = "ecc-sram-aggregate-uncorrectable"
	SRAMHighcorrectableCheckerName       = "ecc-sram-high-correctable"
	SRAMVolatileUncorrectableCheckerName = "ecc-sram-volatile-uncorrectable"
	RemmapedRowsFailureCheckerName       = "remmaped-rows-failure"
	RemmapedRowsUncorrectableCheckerName = "remmaped-rows-high-uncorrectable"
	RemmapedRowsPendingCheckerName       = "remmaped-rows-pending"
	AppClocksCheckerName                 = "app-clocks"
	ClockEventsCheckerName               = "clock-events"
	NvlinkCheckerName                    = "nvlink"
	GpuPersistencedCheckerName           = "persistenced"
	GpuPStateCheckerName                 = "pstate"
	HardwareCheckerName                  = "hardware"
	PCIeCheckerName                      = "pcie"
	SoftwareCheckerName                  = "software"
	GpuTemperatureCheckerName            = "temperature"
	NvlsErrorCheckerName                 = "NVLSError"
)

// GPUCheckItems is a map of check items for GPU
var GPUCheckItems = map[string]common.CheckerResult{
	PCIeACSCheckerName: {
		Name:        PCIeACSCheckerName,
		Description: "Check if PCIe ACS is closed",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "PCIe ACS is closed",
		ErrorName:   "PCIeACSNotClosed",
		Suggestion:  "run `for i in $(lspci | cut -f 1 -d \" \");do setpci -v -s $i ecap_acs+6.w=0;done` to close the ACS. Ideally this will be done automatically online",
	},
	IOMMUCheckerName: {
		Name:        IOMMUCheckerName,
		Description: "Check if IOMMU is closed",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "IOMMU is closed",
		ErrorName:   "IOMMUNotClosed",
		Suggestion:  "To close IOMMU, edit /etc/default/grub and add \"iommu=off\" to the GRUB_CMDLINE_LINUX_DEFAULT line and reboot the system",
	},
	NvPeerMemCheckerName: {
		Name:        NvPeerMemCheckerName,
		Description: "Check if nvidia_peermem is loaded",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "nvidia_peermem is loaded",
		ErrorName:   "NvidiaPeerMemNotLoaded",
		Suggestion:  "run `modprobe nvidia_peermem` to load the nvidia_peermem. Ideally this will be done automatically online",
	},
	NVFabricManagerCheckerName: {
		Name:        NVFabricManagerCheckerName,
		Description: "Check if nvidia-fabricmanager is active",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "nvidia-fabricmanager is active",
		ErrorName:   "NvidiaFabricManagerNotActive",
		Suggestion:  "run `systemctl restart nvidia-fabricmanager` to load the nvidia-fabricmanager. Ideally this will be done automatically online",
	},
	PCIeCheckerName: {
		Name:        PCIeCheckerName,
		Description: "Check if any PCIe link is degraded which is a indicator of performance degrade",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All PCIe links are running at expected speed and width",
		ErrorName:   "PCIeLinkDegraded",
		Suggestion:  "Reboot the system",
	},
	SoftwareCheckerName: {
		Name:        SoftwareCheckerName,
		Description: "Check if all the softwares version are correct",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "Driver and CUDA versions match expected configuration",
		ErrorName:   "SoftwareVersionIncorrect",
		Suggestion:  "Update the software to the expected version",
	},
	GpuTemperatureCheckerName: {
		Name:        GpuTemperatureCheckerName,
		Description: "Check if temperature is larger than specified num (e.g 75 C) which is a indicator of performance degrade",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "HighTemperature",
		Suggestion:  "Observing the performance of application",
	},
	GpuPersistencedCheckerName: {
		Name:        GpuPersistencedCheckerName,
		Description: "Check verifies if the Nvidia GPU persistenced mode is enabled and working correctly",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "NVIDIA Persistenced Mode is enabled",
		ErrorName:   "GPUPersistencedModeNotEnabled",
		Suggestion:  "run `nvidia-persistenced` to auto enable the persistence mode. Ideally this will be done automatically online",
	},
	GpuPStateCheckerName: {
		Name:        GpuPStateCheckerName,
		Description: "Check if the Nvidia GPU performance state is in state 0 -- Maximum Performance",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "GPUStateNotMaxPerformance",
		Suggestion:  "Reset GPU",
	},
	AppClocksCheckerName: {
		Name:        AppClocksCheckerName,
		Description: "Check if all the Nvidia GPUs have set application clocks to max",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "GPU application clocks are set to expected values",
		ErrorName:   "AppClocksNotMax",
		Suggestion:  "run `nvidia-smi -rac` to set the application clocks to max. Ideally this will be done automatically online",
	},
	ClockEventsCheckerName: {
		Name:        ClockEventsCheckerName,
		Description: "Check if any LevelCritical clock event is engaged in any Nvidia GPU",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "No GPU clock throttle events detected",
		ErrorName:   "ClockThrottleEvent",
		Suggestion:  "Diagnostic the GPU for hardware issue",
	},
	NvlinkCheckerName: {
		Name:        NvlinkCheckerName,
		Description: "Check if all the Nvidia GPUs Nvlink are active",
		Status:      consts.StatusNormal,
		Level:       consts.LevelFatal,
		Detail:      "All NVLink are active",
		ErrorName:   "NvlinkNotActive",
		Suggestion:  "Reboot the system",
	},
	HardwareCheckerName: {
		Name:        HardwareCheckerName,
		Description: "Check if any Nvidia GPU is lost",
		Status:      consts.StatusNormal,
		Level:       consts.LevelFatal,
		Detail:      "All GPUs are detected",
		ErrorName:   "GPULost",
		Suggestion:  "Coldreset the system",
	},
	RemmapedRowsUncorrectableCheckerName: {
		Name:        RemmapedRowsUncorrectableCheckerName,
		Description: "Check if any Nvidia GPU has high remmaped rows uncorrectable errors",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "No uncorrectable remapped row errors detected",
		ErrorName:   "HighRemmapedRowsUncorrectableErrors",
		Suggestion:  "Diagnostic the GPU for hardware issue",
	},
	RemmapedRowsPendingCheckerName: {
		Name:        RemmapedRowsPendingCheckerName,
		Description: "Check if any Nvidia GPU has remmaped rows pending",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "No pending remapped rows found",
		ErrorName:   "RemmapedRowsPending",
		Suggestion:  "Reset the GPU device",
	},
	RemmapedRowsFailureCheckerName: {
		Name:        RemmapedRowsFailureCheckerName,
		Description: "Check if any Nvidia GPU has remmaped rows failure",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "No GPU remapped row failure detected",
		ErrorName:   "RemmapedRowsFailure",
		Suggestion:  "Replace the GPU device",
	},
	SRAMVolatileUncorrectableCheckerName: {
		Name:        SRAMVolatileUncorrectableCheckerName,
		Description: "Check if any Nvidia GPU has ecc sram volatile uncorrectable errors",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "No volatile uncorrectable SRAM errors detected",
		ErrorName:   "SRAMVolatileUncorrectableErrors",
		Suggestion:  "Reset the GPU device",
	},
	SRAMAggUncorrectableCheckerName: {
		Name:        SRAMAggUncorrectableCheckerName,
		Description: "Check if any Nvidia GPU has high ecc sram aggregate uncorrectable errors",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "Aggregate Correctable SRAM error rate is within safe threshold",
		ErrorName:   "HighSRAMAggregateUncorrectableErrors",
		Suggestion:  "Replace the GPU device",
	},
	SRAMHighcorrectableCheckerName: {
		Name:        SRAMHighcorrectableCheckerName,
		Description: "Check if any Nvidia GPU has high ecc sram correctable errors",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "Correctable SRAM error rate is within safe threshold",
		ErrorName:   "HighSRAMCorrectableErrors",
		Suggestion:  "Diagnostic the GPU for hardware issue",
	},
}

var CriticalXidEvent = map[uint64]common.CheckerResult{
	31: {
		Name:        "xid-31",
		Description: "GPU memory page fault",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid31-GPUMemoryPageFault",
		Suggestion:  "Reset the GPU device or remind to check if the business code involves illegal memory access operations.",
	},
	48: {
		Name:        "xid-48",
		Description: "DBE (Double Bit Error) ECC Error",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid48-GPUMemoryDBE",
		Suggestion:  "Reset the GPU device",
	},
	63: {
		Name:        "xid-63",
		Description: "ECC page retirement or row remapping recording event",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid63-ECCRowremapperPending",
		Suggestion:  "Reset the GPU device",
	},
	64: {
		Name:        "xid-64",
		Description: "ECC page retirement or row remapper recording failure",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid64-ECCRowremapperFailure",
		Suggestion:  "Reset the GPU device",
	},
	74: {
		Name:        "xid-74",
		Description: "NVLink Error",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid74-NVLinkError",
		Suggestion:  "Reset the GPU device",
	},
	79: {
		Name:        "xid-79",
		Description: "GPU has fallen off the bus",
		Status:      consts.StatusNormal,
		Level:       consts.LevelFatal,
		Detail:      "",
		ErrorName:   "xid79-GPULost",
		Suggestion:  "Coldreset the system",
	},
	92: {
		Name:        "xid-92",
		Description: "High single-bit ECC error rate",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid92-HighSingleBitECCErrorRate",
		Suggestion:  "Replace the GPU device",
	},
	94: {
		Name:        "xid-94",
		Description: "Contained ECC error",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid94-ContainedECCError",
		Suggestion:  "Reset the GPU device",
	},
	95: {
		Name:        "xid-95",
		Description: "Uncontained ECC error",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "xid95-UncontainedECCError",
		Suggestion:  "Replace the GPU device",
	},
}

// IsCriticalXidEvent checks if a given XID is a LevelCritical XID event
func IsCriticalXidEvent(xid uint64) bool {
	_, exists := CriticalXidEvent[xid]
	return exists
}
