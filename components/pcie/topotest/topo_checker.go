package topotest

import (
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

func checkNuma(gpus map[string]*GPUInfo, ibs map[string]*IBInfo, numaConfig []*NumaConfig) *common.CheckerResult {
	res := PciTopoCheckItems[PciTopoNumaCheckerName]
	var builder strings.Builder
	expectedNuma := make(map[string]uint64)
	for _, cfg := range numaConfig {
		for _, bdfItem := range cfg.BdfList {
			expectedNuma[bdfItem.BDF] = cfg.NodeID
		}
	}
	for bdf, gpu := range gpus {
		expectedNumaId, exists := expectedNuma[bdf]
		if !exists {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("Gpu %d (BDF = %s) Not Found In NumaConfig\n",
				gpu.Index, gpu.BDF))
			continue
		}
		if gpu.NumaID != expectedNumaId {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("Gpu %d (BDF=%s) belongs to NUMA ID %d, but expected NUMA ID %d\n",
				gpu.Index, gpu.BDF, gpu.NumaID, expectedNumaId))
		}
	}
	for bdf, ib := range ibs {
		expectedNumaId, exists := expectedNuma[bdf]
		if !exists {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("IB %s (BDF = %s) Not Found In NumaConfig\n",
				ib.Name, ib.BDF))
			continue
		}
		if ib.NumaID != expectedNumaId {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("IB %s (BDF=%s) belongs to NUMA ID %d, but expected NUMA ID %d\n",
				ib.Name, ib.BDF, ib.NumaID, expectedNumaId))
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

	for _, bdfItem := range pciSwitch.GPUList {
		bdfMap[bdfItem.BDF]--
	}
	for _, bdfItem := range pciSwitch.IBList {
		bdfMap[bdfItem.BDF]--
	}

	for _, count := range bdfMap {
		if count != 0 {
			return false
		}
	}
	return true
}

func checkPciSwitches(pciTrees []PciTree, gpus map[string]*GPUInfo, ibs map[string]*IBInfo, PciSwitchesConfig []*PciSwitch) *common.CheckerResult {
	endpointListbyCommonPcieSWs := ParseEndpointsbyCommonSwitch(pciTrees, gpus, ibs)
	res := PciTopoCheckItems[PciTopoSwitchCheckerName]
	switchConfigMap := make(map[string]*PciSwitch)
	for _, cfg := range PciSwitchesConfig {
		switchConfigMap[cfg.SwitchID] = cfg
	}
	var builder strings.Builder
	for _, sw := range endpointListbyCommonPcieSWs {
		switchConfig, exist := switchConfigMap[sw.SwitchBDF]
		if !exist {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("- SwitchBDF %s not found in PciSwitchConfig\n", sw.SwitchBDF))
		} else {
			if !checkDeviceWithCommonSwitch(switchConfig, sw) {
				res.Status = consts.StatusAbnormal
				builder.WriteString(fmt.Sprintf("- PciSwitch %s Check Failed\n  Expected:\n %s\n  Actual:\n %s\n", switchConfig.SwitchID, switchConfig.String(), sw.String()))
			}
		}
	}

	res.Detail = builder.String()
	if res.Status == consts.StatusNormal {
		res.Detail = "Check Pass"
	}
	return &res
}

func CheckGPUTopology(file string) (*common.Result, error) {
	cfg := &PcieTopoConfig{}
	err := cfg.LoadConfig(file)
	if err != nil {
		fmt.Printf("Load GPUTopology Config Err: %v\n", err)
	}
	res := &common.Result{}
	// Build PCIe trees
	nodes, pciTrees, err := BuildPciTrees()
	if err != nil {
		return nil, fmt.Errorf("error building PCIe trees: %v", err)
	}

	gpus := GetGPUList()
	// Find all GPUS by numa node
	FindNvGPUsbyNumaNode(nodes, gpus)

	ibs := GetIBList()

	device, err := config.GetDeviceID()
	var checkRes []*common.CheckerResult
	checkCfg := cfg.PcieTopo[device]

	numaCheckRes := checkNuma(gpus, ibs, checkCfg.NumaConfig)
	checkRes = append(checkRes, numaCheckRes)

	switchCheckRes := checkPciSwitches(pciTrees, gpus, ibs, checkCfg.PciSwitchesConfig)
	checkRes = append(checkRes, switchCheckRes)
	res.Checkers = checkRes
	for _, item := range res.Checkers {
		fmt.Printf("%+v\n", item)
	}
	return res, err
}
