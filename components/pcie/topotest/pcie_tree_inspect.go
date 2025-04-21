package topotest

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

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
type GPUInfo struct {
	BDF                         string //Bus:Device.Function
	UUID                        string //unique identifier
	Index                       int    //GPU sequence number
	NumaID                      uint64 // PCI domain
	DomainID                    string
	SmallestCommonPCIeSwitchBDF string // Smallest PCIe switch BDF of the GPU and its neighboring GPU
}

// PCIeSW represents a PCIe switch and the GPUS connected to it
type PCIeSW struct {
	SwitchBDF string     //BDF of the PCIe switch
	GPUList   []*PciNode // List of GPU nodes connected to this switch
}

// GPUInfoByPCIeSW represents PCIe switch and theGPUS connected to it
type GPUInfoByPCIeSW struct {
	SwitchBDF string     // BDF of the PCIe switch
	GPUList   []*GPUInfo // List of GPU nodes connected to this switch
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
				fmt.Printf("Error parse numaNode code for BDF %s: %v\n", bdf, err)
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

func GetGPUList() map[string]*GPUInfo {
	nvmlInst := nvml.New()
	if ret := nvmlInst.Init(); ret != nvml.SUCCESS {
		panic(fmt.Sprintf("failed to initialize NVML: %v", nvml.ErrorString(ret)))
	}
	defer nvmlInst.Shutdown()
	gpus := make(map[string]*GPUInfo)
	deviceCount, err := nvmlInst.DeviceGetCount()
	if err != nvml.SUCCESS {
		panic(fmt.Sprintf("failed to get device count: %v", nvml.ErrorString(err)))
	}
	for i := 0; i < deviceCount; i++ {
		device, err := nvmlInst.DeviceGetHandleByIndex(i)
		gpu := &GPUInfo{}
		if err != nvml.SUCCESS {
			fmt.Printf("failed to get Nvidia GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		minorNumber, err := device.GetMinorNumber()
		if err != nvml.SUCCESS {
			fmt.Printf("failed to get index for GPU %d: %v", i, nvml.ErrorString(err))
			continue
		}
		gpu.Index = minorNumber
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
		gpu.BDF = fmt.Sprintf("%04x:%02x:%02x.0", pciInfo.Domain, pciInfo.Bus, pciInfo.Device)
		gpu.NumaID = math.MaxUint64              //	Initialize to math.MaxUint64
		gpu.DomainID = "fffff"                   // Initialize to "ffff" string
		gpu.SmallestCommonPCIeSwitchBDF = "ffff" // Initializto "ffff"string
		gpus[gpu.BDF] = gpu
	}
	return gpus
}

// findNvGPUsbyNumaNode identifies all GPU devicesin each numa node
func GetNumaNodeForNvGPUs(nodes map[string]*PciNode, gpus map[string]*GPUInfo) map[uint64][]*GPUInfo {
	gpuListbyNumaNode := make(map[uint64][]*GPUInfo)
	for _, node := range nodes {
		if node.Vendor == 0x10de && node.Class != 0x068000 {

			numaNode := node.NumaID
			domain := strings.Split(node.BDF, ":")[0] // Extract domainfrom BDF
			gpu := gpus[node.BDF]
			gpu.NumaID = numaNode
			gpu.DomainID = domain
			if _, exists := gpuListbyNumaNode[numaNode]; !exists {
				gpuListbyNumaNode[numaNode] = make([]*GPUInfo, 0)
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
	return gpuListbyNumaNode
}

// findNvGPUsbyPcieTree identifies all GPU devices ineach domain root PciTree
func findNvidiaGPUsByPcieTree(pciTree *PciTree) []*PciNode {
	var gpuNodes []*PciNode
	var traverse func(node *PciNode)
	traverse = func(node *PciNode) {
		if node.Vendor == 0x10de && node.Class != 0x068000 {

			gpuNodes = append(gpuNodes, node)
		}
		for _, child := range node.Children {
			traverse(child)
		}
	}
	traverse(pciTree.Root)
	return gpuNodes
}

// FindPathToRoot finds the path to the root for a group of gpu nodes from a given PCIe tree
func FindPathToRoot(gpus []*PciNode) map[string][]*PciNode {
	path := make(map[string][]*PciNode)
	// Traverse upwards to find the root
	for _, gpu := range gpus {
		node := gpu
		path[gpu.BDF] = []*PciNode{}
		for node != nil {
			path[gpu.BDF] = append(path[gpu.BDF], node)
			node = node.Parent
		}
	}
	return path
}

// FindLowestCommonSwitch finds the lowest common switdch for a group of gpu nodes from a given PCIe tree
func findGPULowestCommonSwitch(pciTree *PciTree) []PCIeSW {
	gpus := findNvidiaGPUsByPcieTree(pciTree)
	if len(gpus) == 0 {
		return nil
	}
	paths := FindPathToRoot(gpus)
	gpuBDFs := make([]string, 0, len(gpus))
	pcieSWs := []PCIeSW{}
	pcieSWMap := make(map[string]PCIeSW, 0)
	for bdf := range paths {
		gpuBDFs = append(gpuBDFs, bdf)
	}
	for i := 0; i < len(gpuBDFs)-1; i++ {
		path1 := paths[gpuBDFs[i]]
		for j := i + 1; j < len(gpuBDFs); j++ {
			path2 := paths[gpuBDFs[j]]
			// traverse path1 and path2 to find the first common switch
			var firstCommonSwitch *PciNode
			for m, n := 0, 0; m < len(path1) && n < len(path2); m, n = m+1, n+1 {
				if path1[m].BDF == path2[n].BDF {
					if path1[m].IsSwitch {
						firstCommonSwitch = path1[m]
						if _, exist := pcieSWMap[firstCommonSwitch.BDF]; !exist {
							gpuCommonSwitch := PCIeSW{SwitchBDF: firstCommonSwitch.BDF, GPUList: []*PciNode{gpus[i], gpus[j]}}
							pcieSWMap[firstCommonSwitch.BDF] = gpuCommonSwitch
						} else {
							if i == 0 {
								sw := pcieSWMap[firstCommonSwitch.BDF]
								sw.GPUList = append(sw.GPUList, gpus[j])
								pcieSWMap[firstCommonSwitch.BDF] = sw
							}
						}
						break
					}
				}
			}
		}
	}
	for _, sw := range pcieSWMap {
		pcieSWs = append(pcieSWs, sw)
	}
	return pcieSWs
}

// findCommonSwitch finds the smallest common PCIe switchh for a group of GPUS
func FindNvGPUsbyCommonSwitch(pciTrees []PciTree, gpus map[string]*GPUInfo) []GPUInfoByPCIeSW {
	gpuListbyCommonPcieSWs := []GPUInfoByPCIeSW{}
	for _, pciTree := range pciTrees {
		pcieSWs := findGPULowestCommonSwitch(&pciTree)
		for _, sw := range pcieSWs {
			gpuInfoBySW := GPUInfoByPCIeSW{SwitchBDF: sw.SwitchBDF, GPUList: []*GPUInfo{}}
			for _, gpu := range sw.GPUList {
				_gpu := gpus[gpu.BDF]
				_gpu.SmallestCommonPCIeSwitchBDF = sw.SwitchBDF
				gpuInfoBySW.GPUList = append(gpuInfoBySW.GPUList, gpus[gpu.BDF])
			}
			gpuListbyCommonPcieSWs = append(gpuListbyCommonPcieSWs, gpuInfoBySW)
		}
	}
	return gpuListbyCommonPcieSWs
}

func GetGPUListWithTopoInfo() []*GPUInfo {
	// Build PCIe trees
	nodes, pciTrees, err := BuildPciTrees()
	if err != nil {
		panic(fmt.Sprintf("Error building PCIe trees: %v\n", err))
	}
	// Get GPU Devices
	gpus := GetGPUList()
	// Find all GPUS by numa node
	FindNvGPUsbyNumaNode(nodes, gpus)
	// Find all GPUS by common PCIe switch
	FindNvGPUsbyCommonSwitch(pciTrees, gpus)

	// Convert domainRoots map to slice of PciTree
	gpuList := make([]*GPUInfo, 0, len(gpus))
	for _, gpu := range gpus {
		gpuList = append(gpuList, gpu)
	}
	return gpuList
}