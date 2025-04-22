package topotest

import (
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

func checkNuma(devices map[string]*GPUInfo, numaConfig []*NumaConfig) *common.CheckerResult {
	res := PciTopoCheckItems[PciTopoNumaCheckerName]
	var builder strings.Builder
	for _, cfg := range numaConfig {
		for _, bdf := range cfg.BdfList {
			device, exists := devices[bdf.BDF]
			if !exists {
				res.Status = consts.StatusAbnormal
				builder.WriteString(fmt.Sprintf("numaConfig %s %s Not Found\n",
					bdf.DeviceType, bdf.BDF))
				continue
			}
			if device.NumaID != cfg.NodeID {
				res.Status = consts.StatusAbnormal
				builder.WriteString(fmt.Sprintf("%s %d (BDF=%s) belongs to NUMA ID %d, but expected NUMA ID %d\n",
					bdf.DeviceType, device.Index, device.BDF, device.NumaID, cfg.NodeID))
			}
		}
	}
	res.Detail = builder.String()
	if res.Status == consts.StatusNormal {
		res.Detail = "Check Pass"
	}
	return &res
}

func checkPciSwitches(nodes []*PciNode, PciSwitchesConfig []*PciSwitch) *common.CheckerResult {
	res := PciTopoCheckItems[PciTopoSwitchCheckerName]
	var builder strings.Builder
	nodePaths := FindPathToRoot(nodes)
	for _, cfg := range PciSwitchesConfig {
		pass, err := checkSwitch(cfg.SwitchID, cfg.BdfList, nodePaths)
		if !pass {
			res.Status = consts.StatusAbnormal
			builder.WriteString(fmt.Sprintf("checkPciSwitches %s failed , err:%v\n", cfg.SwitchID, err))
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
	nodes, _, err := BuildPciTrees()
	if err != nil {
		return nil, fmt.Errorf("error building PCIe trees: %v", err)
	}
	// fmt.Printf("%+v", cfg)
	devices := GetGPUList(nodes)
	ibs := GetIBdevs()
	devices = append(devices, ibs...)
	devicesMap := make(map[string]*GPUInfo)
	pciNodes := make([]*PciNode, 0)
	for _, device := range devices {
		devicesMap[device.BDF] = device
		node, exist := nodes[device.BDF]
		if !exist {
			return nil, fmt.Errorf("find no pcieNode for device %s", device.BDF)
		}
		pciNodes = append(pciNodes, node)
	}
	// FillPciDevicesWithCommonSwitch(pciTrees, nodes, devicesMap)
	var checkRes []*common.CheckerResult
	checkCfg := cfg.PcieTopo["0x233010de"]

	numaCheckRes := checkNuma(devicesMap, checkCfg.NumaConfig)
	checkRes = append(checkRes, numaCheckRes)

	switchCheckRes := checkPciSwitches(pciNodes, checkCfg.PciSwitchesConfig)
	checkRes = append(checkRes, switchCheckRes)
	res.Checkers = checkRes
	for _, item := range res.Checkers {
		fmt.Printf("%+v\n", item)
	}
	return res, err
}
