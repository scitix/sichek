package topotest

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

type NumaCount struct {
	GPUCount int
	IBCount  int
}

func checkNuma(devices map[string]*DeviceInfo, numaConfig []*NumaConfig) *common.CheckerResult {
	res := PciTopoCheckItems[PciTopoNumaCheckerName]
	var builder strings.Builder
	numaCount := make(map[uint64]*NumaCount)

	for _, device := range devices {
		if _, ok := numaCount[device.NumaID]; !ok {
			numaCount[device.NumaID] = &NumaCount{}
		}
		stat := numaCount[device.NumaID]

		if device.Type == "GPU" {
			stat.GPUCount++
		} else if device.Type == "IB" {
			stat.IBCount++
		}
		numaCount[device.NumaID] = stat
	}

	for _, config := range numaConfig {
		stat, ok := numaCount[uint64(config.NodeID)]
		if !ok {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("NUMA node %d missing in actual data\n", config.NodeID))
		}
		if stat.GPUCount != config.GPUCount {

			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("NUMA node %d GPU count mismatch: expected %d, got %d\n",
				config.NodeID, config.GPUCount, stat.GPUCount))
		}
		if stat.IBCount != config.IBCount {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("NUMA node %d IB count mismatch: expected %d, got %d\n",
				config.NodeID, config.IBCount, stat.IBCount))
		}
	}

	// check if actual has extra nodes not in expected
	for nodeID := range numaCount {
		found := false
		for _, config := range numaConfig {
			if config.NodeID == nodeID {
				found = true
				break
			}
		}
		if !found {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("unexpected NUMA node %d found in actual data\n", nodeID))
		}
	}

	res.Detail = builder.String()
	if res.Status == consts.StatusNormal {
		res.Detail = "Check Pass"
	}
	return &res
}

func summarizeSwitchConfig(config []*PciSwitch) map[string]int {
	counts := make(map[string]int)
	for _, c := range config {
		key := fmt.Sprintf("gpu_%d&&ib_%d", c.GPU, c.IB)
		counts[key] += c.Count
	}
	return counts
}

func summarizeActualSwitch(pciSwitch map[string]*EndpointInfoByPCIeSW) map[string]int {
	counts := make(map[string]int)
	for _, sw := range pciSwitch {
		gpu, ib := 0, 0
		for _, dev := range sw.DeviceList {
			switch dev.Type {
			case "GPU":
				gpu++
			case "IB":
				ib++
			}
		}
		key := fmt.Sprintf("gpu_%d&&ib_%d", gpu, ib)
		counts[key]++
	}
	return counts
}

func checkPciSwitches(pciTrees []PciTree, nodes map[string]*PciNode, devices map[string]*DeviceInfo, PciSwitchesConfig []*PciSwitch) *common.CheckerResult {
	endpointListbyCommonPcieSWs := ParseEndpointsbyCommonSwitch(pciTrees, nodes, devices)
	res := PciTopoCheckItems[PciTopoSwitchCheckerName]
	var builder strings.Builder

	expected := summarizeSwitchConfig(PciSwitchesConfig)
	actual := summarizeActualSwitch(endpointListbyCommonPcieSWs)

	if !reflect.DeepEqual(expected, actual) {

		res.Status = consts.StatusAbnormal
		builder.WriteString(fmt.Sprintf("switch configuration mismatch.\nExpected: %v\nActual: %v\n", expected, actual))

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
		return nil, fmt.Errorf("load GPUTopology Config Err: %v", err)
	}

	// Build PCIe trees
	nodes, pciTrees, err := BuildPciTrees()
	if err != nil {
		return nil, fmt.Errorf("error building PCIe trees: %v", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("find no pci nodes")
	}
	gpus, err := GetGPUList()
	if err != nil {
		return nil, err
	}
	if len(gpus) == 0 {
		return nil, fmt.Errorf("find no gpus")
	}
	// Find all GPUS by numa node
	FillNvGPUsWithNumaNode(nodes, gpus)

	ibs, err := GetIBList()
	if err != nil {
		return nil, err
	}
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
	res := &common.Result{
		Item:     "Pcie Topo",
		Status:   status,
		Checkers: checkRes,
	}
	return res, err
}

func PrintInfo(result *common.Result, verbos bool) bool {
	if verbos {
		PrintGPUTopology()
	}
	checkerResults := result.Checkers
	if result.Status == consts.StatusNormal {
		fmt.Printf("%sPcie Topo Test Passed%s\n", consts.Green, consts.Reset)
		return true
	}
	for _, result := range checkerResults {
		if result.Status == consts.StatusAbnormal {
			fmt.Printf("%s%s%s\n", consts.Red, result.ErrorName, consts.Reset)
			fmt.Printf("%s\n", result.Detail)
		}
	}
	return false
}
