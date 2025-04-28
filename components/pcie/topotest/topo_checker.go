package topotest

import (
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

func checkNuma(devices map[string]*DeviceInfo, numaConfig []*NumaConfig) *common.CheckerResult {
	res := PciTopoCheckItems[PciTopoNumaCheckerName]
	var builder strings.Builder
	for _, cfg := range numaConfig {
		for _, bdfItem := range cfg.BdfList {
			device, exist := devices[bdfItem.BDF]
			if !exist {
				res.Status = consts.StatusAbnormal
				builder.WriteString(fmt.Sprintf("%s (BDF = %s) Not Found\n", bdfItem.DeviceType, bdfItem.BDF))
				continue
			}
			if device.NumaID != cfg.NodeID {
				res.Status = consts.StatusAbnormal
				builder.WriteString(fmt.Sprintf("%s %s (BDF=%s) belongs to NUMA ID %d, but expected NUMA ID %d\n", device.Type, device.Name, device.BDF, device.NumaID, cfg.NodeID))
			}
		}
	}
	res.Detail = builder.String()
	if res.Status == consts.StatusNormal {
		res.Detail = "Check Pass"
	}
	return &res
}

func checkDeviceWithCommonSwitch(switchConfig *PciSwitch, pciSwitch *EndpointInfoByPCIeSW) bool {
	bdfMap := make(map[string]int)

	for _, bdfItem := range switchConfig.BdfList {
		bdfMap[bdfItem.BDF]++
	}

	for _, bdfItem := range pciSwitch.DeviceList {
		bdfMap[bdfItem.BDF]--
	}

	for _, count := range bdfMap {
		if count != 0 {
			return false
		}
	}
	return true
}

func checkPciSwitches(pciTrees []PciTree, nodes map[string]*PciNode, devices map[string]*DeviceInfo, PciSwitchesConfig []*PciSwitch) *common.CheckerResult {
	endpointListbyCommonPcieSWs := ParseEndpointsbyCommonSwitch(pciTrees, nodes, devices)
	res := PciTopoCheckItems[PciTopoSwitchCheckerName]
	var builder strings.Builder
	for _, cfg := range PciSwitchesConfig {
		endpointInfoByPCIeSW, exist := endpointListbyCommonPcieSWs[cfg.SwitchBDF]
		if !exist {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("- SwitchBDF %s not found\n", endpointInfoByPCIeSW.SwitchBDF))
		} else {
			if !checkDeviceWithCommonSwitch(cfg, endpointInfoByPCIeSW) {
				res.Status = consts.StatusAbnormal
				builder.WriteString(fmt.Sprintf("- PciSwitch %s Check Failed\n  Expected:\n %s\n  Actual:\n %s\n", cfg.SwitchBDF, cfg.String(), endpointInfoByPCIeSW.String()))
			}
		}
	}

	res.Detail = builder.String()
	if res.Status == consts.StatusNormal {
		res.Detail = "Check Pass"
	}
	return &res
}

func CheckGPUTopology(file string, verbos bool) (*common.Result, error) {
	if verbos {
		PrintGPUTopology()
	}
	cfg := &PcieTopoConfig{}
	err := cfg.LoadConfig(file)
	if err != nil {
		return nil, fmt.Errorf("load GPUTopology Config Err: %v", err)
	}
	res := &common.Result{}
	// Build PCIe trees
	nodes, pciTrees, err := BuildPciTrees()
	if err != nil {
		return nil, fmt.Errorf("error building PCIe trees: %v", err)
	}

	gpus := GetGPUList()
	// Find all GPUS by numa node
	FillNvGPUsWithNumaNode(nodes, gpus)

	ibs := GetIBList()
	devices := mergeDeviceMaps(ibs, gpus)
	device, err := config.GetDeviceID()
	if err != nil {
		return nil, fmt.Errorf("get deviceId error: %v", err)
	}
	var checkRes []*common.CheckerResult
	checkCfg, exist := cfg.PcieTopo[device]
	if !exist {
		return nil, fmt.Errorf("device %s topo config not found", device)
	}
	numaCheckRes := checkNuma(devices, checkCfg.NumaConfig)
	checkRes = append(checkRes, numaCheckRes)

	switchCheckRes := checkPciSwitches(pciTrees, nodes, devices, checkCfg.PciSwitchesConfig)
	checkRes = append(checkRes, switchCheckRes)
	status := consts.StatusNormal
	for _, item := range checkRes {
		if item.Status == consts.StatusAbnormal {
			status = consts.StatusAbnormal
		}
	}
	res.Status = status
	res.Checkers = checkRes
	return res, err
}

func PrintInfo(result *common.Result) {
	checkerResults := result.Checkers
	if result.Status == consts.StatusNormal {
		fmt.Printf("%sPcie Topo Test Passed%s\n", consts.Green, consts.Reset)
		return
	}
	for _, result := range checkerResults {
		if result.Status == consts.StatusAbnormal {
			fmt.Printf("%s%s%s\n", consts.Red, result.ErrorName, consts.Reset)
			fmt.Printf("%s\n", result.Detail)
		} else {
			fmt.Printf("%s%s Check Passed %s\n", consts.Green, result.ErrorName, consts.Reset)
		}
	}
}
