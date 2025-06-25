package topotest

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sirupsen/logrus"

	"strings"
)

// PciNode represents a node in the PCIe tree
type PciNode struct {
	BDF      string //Bus:Device.Function
	Name     string
	Vendor   uint64
	Device   uint64
	Class    uint64
	NumaID   uint64
	IsSwitch bool
	Parent   *PciNode
	Children []*PciNode
}

// PciTree represents a PCIe tree for a single domain
type PciTree struct {
	Domain string
	Root   *PciNode //Root node of the tree
}

// GPUInfo represents information about a GPU
type DeviceInfo struct {
	BDF      string //Bus:Device.Function
	UUID     string //unique identifier
	Name     string //GPU :sequence number IB:device name
	NumaID   uint64 // PCI domain
	Type     string
	DomainID string
	BoardID  string
}

// PCIeSW represents a PCIe switch and the GPUS or IBs connected to it
type PCIeSW struct {
	SwitchBDF    string     //BDF of the PCIe switch
	EndpointList []*PciNode // List of GPU or IBs nodes connected to this switch
}

// EndpointInfoByPCIeSW represents PCIe switch and the GPUs / IBs connected to it
type EndpointInfoByPCIeSW struct {
	SwitchBDF  string        // BDF of the PCIe switch
	DeviceList []*DeviceInfo // List of GPU/IB nodes connected to this switch
}

func (sw *EndpointInfoByPCIeSW) String() string {
	var builder strings.Builder
	for _, device := range sw.DeviceList {
		builder.WriteString(fmt.Sprintf(" %s %s: BDF=%v ", device.Type, device.Name, device.BDF))
	}
	return builder.String()
}

// readFile reads the content of a file and returns it as a string
func readFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	var sb strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		sb.WriteString(strings.TrimSpace(scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func GetNUMANodes() []string {
	const path = "/sys/devices/system/node/"
	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Println("Error reading NUMA node info:", err)
		return nil
	}
	var nodes []string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "node") {
			nodes = append(nodes, file.Name())
		}
	}
	return nodes
}

func GetCPUVendorID() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		fmt.Println("Error reading /proc/cpuinfo:", err)
		return "Unknown"
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "vendor_id") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return "Unknown"
}

// BuildPciTrees constructs PCIe trees by reading fFrom /sys/bus/pci/devices
func BuildPciTrees() (map[string]*PciNode, []PciTree, error) {
	// Map to store all nodes by their BDF
	nodes := make(map[string]*PciNode)
	// Iterate over all devices in /sys/bus/pci/ddevices
	deviceDir := "/sys/bus/pci/devices"
	entries, err := os.ReadDir(deviceDir)
	if err != nil {
		return nil, nil, err
	}
	// Parse device entries
	for _, entry := range entries {
		bdf := entry.Name()
		// Initialize a new PCI node
		node := &PciNode{BDF: bdf}
		// Read device class
		classPath := filepath.Join(deviceDir, bdf, "class")
		classStr, err := readFile(classPath)
		var classCode uint64
		if err == nil {
			classCode, err = strconv.ParseUint(classStr, 0, 32)
			if err != nil {
				fmt.Printf("Error parse class code for BDF %s: %v\n", bdf, err)
				continue
			}
			node.IsSwitch = (classCode == 0x060400) // 0x060400:PCI-to-PCI bridge

		} else {
			fmt.Printf("Error reading class for BDF %s: %v\n", bdf, err)
			continue
		}
		vendorPath := filepath.Join(deviceDir, bdf, "vendor")
		vendorStr, err := readFile(vendorPath)
		var vendorCode uint64
		if err == nil {
			vendorCode, err = strconv.ParseUint(vendorStr, 0, 32)
			if err != nil {
				fmt.Printf("Error parse vendor code for BDF %s: %v\n", bdf, err)
				continue
			}
		} else {
			fmt.Printf("Error reading vendor for BDF %s: %v\n", bdf, err)
			continue
		}
		devicePath := filepath.Join(deviceDir, bdf, "device")
		deviceStr, err := readFile(devicePath)
		var deviceCode uint64
		if err == nil {
			deviceCode, err = strconv.ParseUint(deviceStr, 0, 32)
			if err != nil {
				fmt.Printf("Error parse device code for BDF %ss: %v\n", bdf, err)
				continue
			}
		} else {
			fmt.Printf("Error reading device for BDF %s: %v\n", bdf, err)
			continue
		}

		numaNodePath := filepath.Join(deviceDir, bdf, "numa_node")
		numaNodeStr, err := readFile(numaNodePath)
		var numaNode uint64
		if err == nil {
			numaNode, err = strconv.ParseUint(numaNodeStr, 0, 32)
			if err != nil {
				continue
			}
		} else {
			fmt.Printf("Error reading numaNode for BDF %s:%v\n", bdf, err)
			continue
		}
		node.Name = fmt.Sprintf("Vendor-%s-Device-%s", vendorStr, deviceStr)
		node.Vendor = vendorCode
		node.Device = deviceCode
		node.Class = classCode
		node.NumaID = numaNode
		nodes[bdf] = node
	}
	// Establish parent-child relationships
	for bdf, node := range nodes {
		parentPath := deviceDir + "/" + bdf + "/.."
		parentRealPath, err := filepath.EvalSymlinks(parentPath)
		if err != nil {
			fmt.Printf("Warning: failed to evaluate symlink ffor %s: %v\n", bdf, err)
			continue
		}
		parentBDF := filepath.Base(parentRealPath)
		if parentNode, exists := nodes[parentBDF]; exists {
			node.Parent = parentNode
			parentNode.Children = append(parentNode.Children, node)
		}
	}
	// Construct domainRoots to include only PCI Briddges
	domainRoots := make(map[string][]*PciNode)
	for _, node := range nodes {
		if node.Parent == nil {
			domain := strings.Split(node.BDF, ":")[0] // Extract domain
			if _, exists := domainRoots[domain]; !exists {
				// fmt.Printf("Domain: %s\n", domain)
				domainRoots[domain] = []*PciNode{}
			}
			if node.IsSwitch {
				domainRoots[domain] = append(domainRoots[domain], node)
				// fmt.Printf("Domain: %s, pcie switch = %s, child= %v\n",domain, node.BDF, len(node.Children)
			}
		}
	}
	// Convert domainRoots map to slice of PciTree
	var pciTrees []PciTree
	for domain, root := range domainRoots {
		for _, r := range root {
			pciTrees = append(pciTrees, PciTree{
				Domain: domain,
				Root:   r,
			})
		}
	}
	return nodes, pciTrees, nil
}

func GetGPUList() (map[string]*DeviceInfo, error) {
	nvmlInst := nvml.New()
	if ret := nvmlInst.Init(); ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	defer nvmlInst.Shutdown()
	gpus := make(map[string]*DeviceInfo)
	deviceCount, err := nvmlInst.DeviceGetCount()
	if err != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(err))
	}
	for i := 0; i < deviceCount; i++ {
		device, err := nvmlInst.DeviceGetHandleByIndex(i)
		gpu := &DeviceInfo{}
		if err != nvml.SUCCESS {
			fmt.Printf("failed to get Nvidia GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		minorNumber, err := device.GetMinorNumber()
		if err != nvml.SUCCESS {
			fmt.Printf("failed to get index for GPU %d: %v", i, nvml.ErrorString(err))
			continue
		}
		gpu.Name = strconv.Itoa(minorNumber)
		// Get GPU UUID
		uuid, err := device.GetUUID()
		if err != nvml.SUCCESS {
			fmt.Printf("failed to get UUID for GPU %d: %v", i, nvml.ErrorString(err))
			continue
		}
		gpu.UUID = uuid
		pciInfo, err := device.GetPciInfo()
		if err != nvml.SUCCESS {
			fmt.Printf("failed to get PCIe Info for NVIDIA GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		gpu.Type = "GPU"
		gpu.BDF = fmt.Sprintf("%04x:%02x:%02x.0", pciInfo.Domain, pciInfo.Bus, pciInfo.Device)
		gpu.NumaID = math.MaxUint64 //	Initialize to math.MaxUint64
		gpu.DomainID = "fffff"      // Initialize to "ffff" string
		gpus[gpu.BDF] = gpu
	}
	return gpus, nil
}

func GetIBList() (map[string]*DeviceInfo, error) {
	basePath := "/sys/class/infiniband"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read infiniband dir: %v", err)
	}

	ibInfos := make(map[string]*DeviceInfo)
	for _, entry := range entries {
		name := entry.Name()

		devPath := filepath.Join(basePath, name, "device")
		vfPath := filepath.Join(basePath, name, "device", "physfn")
		if _, err := os.Stat(vfPath); err == nil {
			fmt.Printf("Skipping virtual function for %s\n", name)
			continue // Skip virtual functions
		}
		// read PCI BDF
		realPath, err := filepath.EvalSymlinks(devPath)
		if err != nil {
			fmt.Printf("Error evaluating symlink for %s: %v\n", devPath, err)
			continue
		}
		bdf := filepath.Base(realPath)

		// read board ID
		boardIDPath := filepath.Join(basePath, name, "board_id")
		boardIDStr, err := readFile(boardIDPath)
		if err == nil {
			boardIDStr = strings.TrimSpace(boardIDStr)
		} else {
			fmt.Printf("Error reading board ID for BDF %s: %v\n", bdf, err)
			continue
		}

		// get domain id
		parts := strings.Split(bdf, ":")
		if len(parts) != 3 {
			logrus.Errorf("invalid BDF format: %s", bdf)
			continue
		}
		domainID := parts[0]

		// read NUMA node
		numaNodePath := filepath.Join(devPath, "numa_node")
		numaNodeStr, err := readFile(numaNodePath)
		var numaNode uint64
		if err == nil {
			numaNode, err = strconv.ParseUint(numaNodeStr, 0, 32)
			if err != nil {
				fmt.Printf("Error parse IB's numaNode code for BDF %s: %v\n", bdf, err)
				continue
			}
		} else {
			fmt.Printf("Error reading numaNode for BDF %s:%v\n", bdf, err)
			continue
		}
		fmt.Printf("Found IB device: %s, boardID=%s, BDF=%v, numa_node=%v, domain=%v\n", name, boardIDStr, bdf, numaNode, domainID)
		ibInfo := &DeviceInfo{
			Type:     "IB",
			BDF:      bdf,
			BoardID:  boardIDStr,
			Name:     name,
			NumaID:   numaNode,
			DomainID: domainID,
		}
		ibInfos[bdf] = ibInfo
	}
	return ibInfos, nil
}

// FillNvGPUsWithNumaNode identifies all GPU devices in each numa node
func FillNvGPUsWithNumaNode(nodes map[string]*PciNode, gpus map[string]*DeviceInfo) {
	gpuListbyNumaNode := make(map[uint64][]*DeviceInfo)
	for _, node := range nodes {
		if node.Vendor == 0x10de && node.Class != 0x068000 {
			numaNode := node.NumaID
			domain := strings.Split(node.BDF, ":")[0] // Extract domainfrom BDF
			gpu, exist := gpus[node.BDF]
			if !exist {
				fmt.Printf("not find gpus for BDF %s\n", node.BDF)
				continue
			}
			gpu.NumaID = numaNode
			gpu.DomainID = domain
			if _, exists := gpuListbyNumaNode[numaNode]; !exists {
				gpuListbyNumaNode[numaNode] = make([]*DeviceInfo, 0)
			}
			gpuListbyNumaNode[numaNode] = append(gpuListbyNumaNode[numaNode], gpu)
		}
	}
	// especial case: for AMD Server, if there are 8 numa nodes annd two GPU in the same numa node, then let one of them be in the numa node minus 1
	cpuVendorId := GetCPUVendorID()
	numaNodes := GetNUMANodes()
	if cpuVendorId == "AuthenticAMD" && len(numaNodes) == 8 {
		for _, gpus := range gpuListbyNumaNode {
			if len(gpus) == 2 {
				if gpus[0].NumaID == gpus[1].NumaID {
					gpus[0].NumaID = gpus[0].NumaID - 1
				}
			}
			for _, gpu := range gpus {
				if gpu.NumaID < 4 {
					gpu.DomainID = "0000"
				} else {
					gpu.DomainID = "0001"
				}
			}
		}
	}
}

// FindPathToRoot finds the path to the root for a group of endpoints nodes (GPU or IB) from a given PCIe tree
func FindPathToRoot(endpoints []*PciNode) map[string][]*PciNode {
	path := make(map[string][]*PciNode)
	// Traverse upwards to find the root
	for _, gpu := range endpoints {
		node := gpu
		path[gpu.BDF] = []*PciNode{}
		for node != nil {
			path[gpu.BDF] = append(path[gpu.BDF], node)
			node = node.Parent
		}
	}
	return path
}

// FindLowestCommonSwitch finds the lowest common switdch for a group of endpoints nodes (GPU or IB) from a given PCIe tree
func findEndpointLowestCommonSwitch(pciTree *PciTree, endpoints []*PciNode) map[string]map[string]struct{} {
	if len(endpoints) == 0 {
		return nil
	}
	paths := FindPathToRoot(endpoints)
	endpointBDFs := make([]string, 0, len(endpoints))
	swIDToBDFMap := make(map[string]map[string]struct{})
	for bdf := range paths {
		endpointBDFs = append(endpointBDFs, bdf)
	}
	for i := 0; i < len(endpointBDFs); i++ {
		path1 := paths[endpointBDFs[i]]
		swMap := make(map[string]int)
		var minSwitch []string
		minSwitchIdx := math.MaxInt
		for idx, node := range path1 {
			if node.IsSwitch {
				swMap[node.BDF] = idx
			}
		}
		for j := 0; j < len(endpointBDFs); j++ {
			if i == j {
				continue
			}
			path2 := paths[endpointBDFs[j]]
			for _, node := range path2 {
				if idx, exist := swMap[node.BDF]; exist {
					if idx < minSwitchIdx {
						minSwitchIdx = idx
						minSwitch = []string{endpointBDFs[j]}
					} else if idx == minSwitchIdx {
						minSwitch = append(minSwitch, endpointBDFs[j])
					}
					break
				}
			}

		}
		if minSwitchIdx == math.MaxInt {
			// logrus.WithField("component", "pcie").Warnf("find no minSwitchIdx for %s", endpointBDFs[i])
			continue
		}
		sw_id := path1[minSwitchIdx].BDF
		if _, exists := swIDToBDFMap[sw_id]; !exists {
			swIDToBDFMap[sw_id] = make(map[string]struct{})
		}
		swIDToBDFMap[sw_id][endpointBDFs[i]] = struct{}{}
		for _, bdf := range minSwitch {
			swIDToBDFMap[sw_id][bdf] = struct{}{}
		}
	}
	return swIDToBDFMap
}

// findCommonSwitch finds the smallest common PCIe switchs for a group of endpoints, likes GPUs/IBs
func ParseEndpointsbyCommonSwitch(pciTrees []PciTree, nodes map[string]*PciNode, devices map[string]*DeviceInfo) map[string]*EndpointInfoByPCIeSW {
	endpointListbyCommonPcieSWs := make(map[string]*EndpointInfoByPCIeSW)
	endpoints := make([]*PciNode, 0)
	for _, device := range devices {
		if node, exist := nodes[device.BDF]; exist {
			endpoints = append(endpoints, node)
		}
	}
	for _, pciTree := range pciTrees {
		swIDToBDFMap := findEndpointLowestCommonSwitch(&pciTree, endpoints)
		for swId, bdfSet := range swIDToBDFMap {
			deviceInfoBySW := &EndpointInfoByPCIeSW{SwitchBDF: swId, DeviceList: []*DeviceInfo{}}
			for bdf := range bdfSet {
				if gpu, exist := devices[bdf]; exist {
					deviceInfoBySW.DeviceList = append(deviceInfoBySW.DeviceList, gpu)
				}
			}
			endpointListbyCommonPcieSWs[swId] = deviceInfoBySW
		}
	}
	return endpointListbyCommonPcieSWs
}

func mergeDeviceMaps(map1, map2 map[string]*DeviceInfo) map[string]*DeviceInfo {
	merged := make(map[string]*DeviceInfo)

	for key, value := range map1 {
		merged[key] = value
	}

	for key, value := range map2 {
		if existingValue, exists := merged[key]; exists {
			fmt.Printf("Key %s exists, keeping original value: %+v\n", key, existingValue)
		} else {
			merged[key] = value
		}
	}
	return merged
}

func PrintGPUTopology() {
	cpuVendorId := GetCPUVendorID()
	fmt.Printf("CPU vendor id: %s\n", cpuVendorId)
	numaNodes := GetNUMANodes()
	fmt.Printf("Number of NUMA nodes: %d\n", len(numaNodes))
	if cpuVendorId == "AuthenticAMD" {
		fmt.Printf("Get AuthenticAMD with %d NUMA nodes\n", len(numaNodes))
	}
	// Build PCIe trees
	nodes, pciTrees, err := BuildPciTrees()
	if err != nil {
		// t.Errorf("Error building PCIe trees: %v\n", err)
		fmt.Printf("Error building PCIe trees: %v\n", err)
		os.Exit(1)
	}

	ibs, err := GetIBList()
	if err != nil {
		fmt.Printf("Error GetIBList: %v\n", err)
		return
	}
	for _, ib := range ibs {
		fmt.Printf("IB %s: boardID=%s, BDF=%v, numa_node=%v, domain=%v\n", ib.Name, ib.BoardID, ib.BDF, ib.NumaID, ib.DomainID)
	}
	gpus, err := GetGPUList()
	if err != nil {
		fmt.Printf("Error GetGPUList: %v\n", err)
		return
	}
	// Find all GPUS by numa node
	FillNvGPUsWithNumaNode(nodes, gpus)
	for _, gpu := range gpus {
		fmt.Printf("GPU %s: uuid=%v, BDF=%v, numa_node=%v, domain=%v\n", gpu.Name, gpu.UUID, gpu.BDF, gpu.NumaID, gpu.DomainID)
	}
	devices := mergeDeviceMaps(ibs, gpus)
	// Find all GPUS and IBs by common PCIe switch
	endpointListbyCommonPcieSWs := ParseEndpointsbyCommonSwitch(pciTrees, nodes, devices)
	fmt.Printf("Find GPUS and IBs by common PCIe switch: \n")
	for _, sw := range endpointListbyCommonPcieSWs {
		fmt.Printf(" - PCIe Switch: %s, with GPUS and IBs: \n", sw.SwitchBDF)
		for _, device := range sw.DeviceList {
			fmt.Printf("%s %s: uuid=%v, BDF=%v, numa_node=%v, domain=%v\n", device.Type, device.Name, device.UUID, device.BDF, device.NumaID, device.DomainID)
		}
		fmt.Println()
	}

}
