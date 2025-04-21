package topotest

import (
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

func checkNuma(gpuList []*GPUInfo, numaConfig []*NumaConfig) *common.CheckerResult {
	res := PciTopoCheckItems[PciTopoNumaCheckerName]
	bdfToNuma := make(map[string]uint64)
	for _, cfg := range numaConfig {
		for _, bdf := range cfg.BdfList {
			bdfToNuma[bdf] = cfg.NodeID
		}
	}
	var builder strings.Builder
	for _, gpu := range gpuList {
		expectNumaId, exists := bdfToNuma[gpu.BDF]
		if !exists {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("GPU %d Not In NumaConfig (uuid=%s, BDF=%s, numa_node=%d)\n",
				gpu.Index, gpu.UUID, gpu.BDF, gpu.NumaID))
			continue
		}

		if gpu.NumaID != expectNumaId {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("GPU %d (BDF=%s) belongs to NUMA ID %d, but expected NUMA ID %d\n",
				gpu.Index, gpu.BDF, gpu.NumaID, expectNumaId))
		}
	}
	res.Detail = builder.String()
	if res.Status == consts.StatusNormal {
		res.Detail = "check pass"
	}
	return &res
}

func checkPciSwitches(gpuList []*GPUInfo, PciSwitchesConfig []*PciSwitch) *common.CheckerResult {
	res := PciTopoCheckItems[PciTopoSwitchCheckerName]
	bdfToSwitch := make(map[string]string)
	for _, cfg := range PciSwitchesConfig {
		for _, bdf := range cfg.BdfList {
			bdfToSwitch[bdf] = cfg.SwitchID
		}
	}
	var builder strings.Builder
	for _, gpu := range gpuList {
		expectSwitchID, exists := bdfToSwitch[gpu.BDF]
		if !exists {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("GPU %d Not In SwitchConfig (BDF = %s, device_id = %s, switch_id = %s)\n",
				gpu.Index, gpu.BDF, gpu.PciDeviceID, gpu.SmallestCommonPCIeSwitchBDF))
			continue
		}

		if gpu.SmallestCommonPCIeSwitchBDF != expectSwitchID {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("GPU %d (BDF = %s, device_id = %s) belongs to Switch ID %s, but expected Switch ID %s\n",
				gpu.Index, gpu.BDF, gpu.PciDeviceID, gpu.SmallestCommonPCIeSwitchBDF, expectSwitchID))
		}
	}
	res.Detail = builder.String()
	if res.Status == consts.StatusNormal {
		res.Detail = "Check Pass"
	}
	return &res
}

func CheckGPUTopology(file string) ([]*common.CheckerResult, error) {
	cfg := &PciTopoConfig{}
	err := cfg.LoadConfig(file)
	if err != nil {
		fmt.Printf("Load GPUTopology Config Err: %v\n", err)
	}
	var res []*common.CheckerResult
	gpus := GetGPUList()
	// Build PCIe trees
	nodes, pciTrees, err := BuildPciTrees()
	if err != nil {
		return nil, fmt.Errorf("error building PCIe trees: %v", err)
	}
	FillNvGPUsWithNumaNode(nodes, gpus)
	FillNvGPUsWithCommonSwitch(pciTrees, gpus)

	deviceToGpus := make(map[string][]*GPUInfo)
	for _, gpu := range gpus {
		if _, ok := deviceToGpus[gpu.PciDeviceID]; !ok {
			deviceToGpus[gpu.PciDeviceID] = make([]*GPUInfo, 0)
		}
		deviceToGpus[gpu.PciDeviceID] = append(deviceToGpus[gpu.PciDeviceID], gpu)
	}
	for deviceId, gpuList := range deviceToGpus {
		cfg, exists := cfg.DeviceConfig[deviceId]
		if !exists {
			return nil, fmt.Errorf("get DeviceConfig for deviceId %s err: %v", deviceId, err)
		}
		numaCheckRes := checkNuma(gpuList, cfg.NumaConfig)
		res = append(res, numaCheckRes)
		switchCheckRes := checkPciSwitches(gpuList, cfg.PciSwitchesConfig)
		res = append(res, switchCheckRes)
	}
	for _, item := range res {
		fmt.Printf("%+v\n", item)
	}
	return res, nil
}
