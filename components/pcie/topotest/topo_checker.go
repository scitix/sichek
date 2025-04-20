package topotest

import (
	"fmt"
	"os"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

func checkNuma(gpus map[string]*GPUInfo, nodes map[string]*PciNode, numaConfig []*NumaConfig) common.CheckerResult {
	res := PciTopoCheckItems[PciTopoNumaCheckerName]
	// Find all GPUS by numa node
	gpuNodesbyNumaNode := FindNvGPUsbyNumaNode(nodes, gpus)
	for _, cfg := range numaConfig {
		gpuList := gpuNodesbyNumaNode[uint64(cfg.NodeID)]
		bdfMap := make(map[string]struct{})
		for _, gpu := range gpuList {
			bdfMap[gpu.BDF] = struct{}{}
		}
		for _, bdf := range cfg.BdfList {
			if _, exists := bdfMap[bdf]; !exists {
				res.Status = consts.StatusAbnormal
				fmt.Printf("GPU %d: uuid=%v, BDF=%v, numa_node=%v, domain=%v, sw_id=%v\n", gpu.Index, gpu.UUID, gpu.BDF, gpu.NumaID, gpu.DomainID, gpu.SmallestCommonPCIeSwitchBDF)
	
				break
			}
		}
	}
	if res.Status == consts.StatusAbnormal{

	}
	return res
}
func checkPciSwitches() {

}
func CheckDevice(cfg map[string]*PciDeviceTopoConfig) {
	gpus := GetGPUList()
	for _, gpu := range gpus {
		fmt.Printf("GPU %d: uuid=%v, BDF=%v, numa_node=%v, domain=%v, sw_id=%v\n", gpu.Index, gpu.UUID, gpu.BDF, gpu.NumaID, gpu.DomainID, gpu.SmallestCommonPCIeSwitchBDF)
	}
	// Build PCIe trees
	nodes, pciTrees, err := BuildPciTrees()
	if err != nil {
		// t.Errorf("Error building PCIe trees: %v\n", err)
		fmt.Printf("Error building PCIe trees: %v\n", err)
		os.Exit(1)
	}
	// Find all GPUS by common PCIe switch
	gpuNodesbyCommonPcieSWs := FindNvGPUsbyCommonSwitch(pciTrees, gpus)
	fmt.Printf("Find GPUS by common PCIe switch: \n")
	for _, sw := range gpuNodesbyCommonPcieSWs {
		fmt.Printf(" - PCIe Switch: %s, with GPUS: \n", sw.SwitchBDF)
		for _, gpu := range sw.GPUList {
			fmt.Printf("GPU %d: uuid=%v, BDF=%v, numa_node=%v, domain=%v, sw_id=%v\n", gpu.Index, gpu.UUID, gpu.BDF, gpu.NumaID, gpu.DomainID, gpu.SmallestCommonPCIeSwitchBDF)
		}
		fmt.Println()
	}
	gpusList := GetGPUListWithTopoInfo()
	for _, gpu := range gpusList {
		fmt.Printf("GPU %d: uuid=%v, BDF=%v, numa_node=%v, domain=%v, sw_id=%v\n", gpu.Index, gpu.UUID, gpu.BDF, gpu.NumaID, gpu.DomainID, gpu.SmallestCommonPCIeSwitchBDF)
	}
}

func CheckGPUTopology() {
	cfg := &PciTopoConfig{}
	err := cfg.LoadConfig("")
	if err != nil {
		fmt.Printf("Load GPUTopology Config Err: %v\n", err)
	}

	// cpuVendorId := GetCPUVendorID()
	// fmt.Printf("CPU vendor id: %s\n", cpuVendorId)
	// numaNodes := GetNUMANodes()
	// fmt.Printf("Number of NUMA nodes: %d\n", len(numaNodes))
	// if cpuVendorId == "AuthenticAMD" {
	// 	fmt.Printf("Get AuthenticAMD with %d NUMA nodes\n", len(numaNodes))
	// }

	// fmt.Println()

}
