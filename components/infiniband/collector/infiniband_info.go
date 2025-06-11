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
package collector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

var (
	IBSYSPathPre = "/sys/class/infiniband/"
	PCIPath      = "/sys/bus/pci/devices/"
)

type InfinibandInfo struct {
	HCAPCINum      int                          `json:"hca_pci_num"`
	IBDevs         map[string]string            `json:"ib_dev"`
	IBHardWareInfo []IBHardWareInfo             `json:"ib_hardware_info"`
	IBSoftWareInfo IBSoftWareInfo               `json:"ib_software_info"`
	PCIETreeInfo   []PCIETreeInfo               `json:"pcie_tree_info"`
	IBCounters     map[string]map[string]uint64 `json:"ib_counters"`
	Time           time.Time                    `json:"time"`
}

type IBHardWareInfo struct {
	IBDev        string `json:"IBdev"`
	NetDev       string `json:"net_dev"`
	HCAType      string `json:"hca_type"`
	SystemGUID   string `json:"system_guid"`
	NodeGUID     string `json:"node_guid"`
	PhyState     string `json:"phy_state"`
	PortState    string `json:"port_state"`
	LinkLayer    string `json:"link_layer"`
	NetOperstate string `json:"net_operstate"`
	PortSpeed    string `json:"port_speed"`
	BoardID      string `json:"board_id"`
	DeviceID     string `json:"device_id"`
	PCIEBDF      string `json:"pcie_bdf"`
	PCIESpeed    string `json:"pcie_speed"`
	PCIEWidth    string `json:"pcie_width"`
	PCIEMRR      string `json:"pcie_mrr"`
	Slot         string `json:"slot"`
	NumaNode     string `json:"numa_node"`
	CPULists     string `json:"cpu_lists"`
	FWVer        string `json:"fw_ver"`
	VPD          string `json:"vpd"`
	OFEDVer      string `json:"ofed_ver"` // compatible with IB Spec Requirement
}

type PCIETreeInfo struct {
	PCIETreeSpeed []PCIETreeSpeedInfo `json:"pcie_tree_speed"`
	PCIETreeWidth []PCIETreeWidthInfo `json:"pcie_tree_width"`
}

type PCIETreeSpeedInfo struct {
	BDF   string `json:"bdf"`
	Speed string `json:"speed"`
}

type PCIETreeWidthInfo struct {
	BDF   string `json:"bdf"`
	Width string `json:"width"`
}

type IBSoftWareInfo struct {
	OFEDVer      string   `json:"ofed_ver"`
	KernelModule []string `json:"kernel_module"`
}

func (i *InfinibandInfo) JSON() (string, error) {
	data, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (i *InfinibandInfo) GetPFDevs(IBDev []string) []string {
	PFDevs := make([]string, 0)
	for _, IBDev := range IBDev {
		sriovTotalVFsPath := path.Join(IBSYSPathPre, IBDev, "device/sriov_totalvfs")
		_, err := os.Stat(sriovTotalVFsPath)
		if err == nil {
			PFDevs = append(PFDevs, IBDev)
		}
	}
	return PFDevs
}

func (i *InfinibandInfo) GetIBdevs() map[string]string {
	allIBDevs := GetFileCnt(IBSYSPathPre)
	PFDevs := i.GetPFDevs(allIBDevs)

	IBDevs := make(map[string]string)
	for _, IBDev := range PFDevs {
		if strings.Contains(IBDev, "bond") {
			continue
		}
		netPath := filepath.Join(IBSYSPathPre, IBDev, "device/net")
		if _, err := os.Stat(netPath); os.IsNotExist(err) {
			logrus.WithField("component", "infiniband").Infof("No net directory found for IB device %s, skipping", IBDev)
			continue
		}
		ibNetDev := GetFileCnt(netPath)[0]
		IBDevs[IBDev] = ibNetDev
	}

	return IBDevs
}

func (i *InfinibandInfo) GetIBdev2NetDev(IBDev string) []string {
	return i.GetSysCnt(IBDev, "device/net")
}

func (i *InfinibandInfo) GetNetOperstate(IBDev string) string {
	netPath := filepath.Join(IBSYSPathPre, IBDev, "device", "net")
	dirs, err := os.ReadDir(netPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to read net directory %s: %v", netPath, err)
		return ""
	}
	if len(dirs) == 0 {
		logrus.WithField("component", "infiniband").Errorf("no network interfaces found under %s", netPath)
		return ""
	}
	netDir := dirs[0].Name()
	operstatePath := filepath.Join(netPath, netDir, "operstate")
	data, err := os.ReadFile(operstatePath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to read %s: %v", operstatePath, err)
		return ""
	}
	return strings.TrimSpace(string(data))
}

func listFiles(dir, IBDev, counterType string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		logrus.WithField("component", "infiniband").Infof("Fail to Read dir:%s", dir)
		return nil, err
	}

	var fileNames []string
	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}
	return fileNames, nil
}

func GetIBCounter(IBDev string, counterType string) (map[string]uint64, error) {
	Counters := make(map[string]uint64, 0)
	counterPath := path.Join(IBSYSPathPre, IBDev, "ports/1", counterType)
	ibCounterName, err := listFiles(counterPath, IBDev, counterType)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Fail to get the counter from path :%s", counterPath)
		return nil, err
	}
	for _, counter := range ibCounterName {

		counterValuePath := path.Join(counterPath, counter)
		contents, err := os.ReadFile(counterValuePath)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Fail to read the ib counter from path: %s", counterValuePath)
		}
		// counter Value
		value, err := strconv.ParseUint(strings.ReplaceAll(string(contents), "\n", ""), 10, 64)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Error covering string to uint64")
			return nil, err
		}
		Counters[counter] = value
	}

	return Counters, nil
}

func (i *InfinibandInfo) Name() string {
	return i.Name()
}

func (i *InfinibandInfo) GetIBCounters(IBDev string) map[string]uint64 {
	Counters := make(map[string]uint64, 0)
	var wg sync.WaitGroup
	counterTypes := []string{"counters", "hw_counters"}

	wg.Add(len(counterTypes))
	for _, counterType := range counterTypes {
		go func(ct string) {
			defer wg.Done()
			var err error
			Counters, err = GetIBCounter(IBDev, ct)
			if err != nil {
				logrus.WithField("component", "infiniband").Errorf("Get IB Counter failed, err:%s", err)
				return
			}
		}(counterType)
	}
	wg.Wait()
	return Counters
}

func (i *InfinibandInfo) GetHCAType(IBDev string) []string {
	return i.GetSysCnt(IBDev, "hca_type")
}

func (i *InfinibandInfo) GetVPD(IBDev string) string {
	vpdPath := filepath.Join(IBSYSPathPre, IBDev, "device", "vpd")
	data, err := os.ReadFile(vpdPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to read vpd file %s, err: %v", vpdPath, err)
	}
	re := regexp.MustCompile(`[ -~]+`)
	match := re.Find(data)
	if match != nil {
		return string(match)
	} else {
		logrus.WithField("component", "infiniband").Errorf("failed to get oem info from vpd file %s", vpdPath)
		return ""
	}
}

func (i *InfinibandInfo) GetFWVer(IBDev string) []string {
	return i.GetSysCnt(IBDev, "fw_ver")
}

func (i *InfinibandInfo) GetBoardID(IBDev string) []string {
	return i.GetSysCnt(IBDev, "board_id")
}

func (i *InfinibandInfo) GetPhyStat(IBDev string) []string {
	return i.GetSysCnt(IBDev, "ports/1/phys_state")
}

func (i *InfinibandInfo) GetIBStat(IBDev string) []string {
	return i.GetSysCnt(IBDev, "ports/1/state")
}

func (i *InfinibandInfo) GetPortSpeed(IBDev string) []string {
	return i.GetSysCnt(IBDev, "ports/1/rate")
}

func (i *InfinibandInfo) GetLinkLayer(IBDev string) []string {
	return i.GetSysCnt(IBDev, "ports/1/link_layer")
}

func (i *InfinibandInfo) GetPCIEMRR(ctx context.Context, IBDev string) []string {
	bdf := i.GetBDF(IBDev)

	// lspciCmd := exec.Command("lspci", "-s", bdf[0], "-vvv")
	lspciOutput, err := utils.ExecCommand(ctx, "lspci", "-s", bdf[0], "-vvv")
	if err != nil {
		return nil
	}

	grepCmd := exec.Command("grep", "MaxReadReq")
	grepCmd.Stdin = bytes.NewBuffer(lspciOutput)
	grepOutput, err := grepCmd.Output()
	if err != nil {
		return nil
	}

	parts := strings.Split(string(grepOutput), "MaxReadReq ")
	var mrr []string
	if len(parts) > 1 {
		mrr = strings.Fields(parts[1])
	}

	return mrr
}

func (i *InfinibandInfo) GetPCIETreeSpeed(IBDev string) []PCIETreeSpeedInfo {
	bdf := i.GetBDF(IBDev)[0]
	devicePath := filepath.Join(PCIPath, bdf)
	cmd := exec.Command("readlink", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	bdfRegexPattern := `\b[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]\b`
	re := regexp.MustCompile(bdfRegexPattern)
	bdfs := re.FindAllString(string(output), -1)
	allTreeSpeed := make([]PCIETreeSpeedInfo, 0, len(bdfs))

	for _, bdf := range bdfs {
		var perTreeSpeed PCIETreeSpeedInfo
		speed := GetFileCnt(filepath.Join(PCIPath, bdf, "current_link_speed"))
		logrus.WithField("component", "infiniband").Infof("get the pcie tree speed, ib:%s bdf:%s speed:%s", IBDev, bdf, speed[0])
		perTreeSpeed.BDF = bdf
		perTreeSpeed.Speed = speed[0]
		allTreeSpeed = append(allTreeSpeed, perTreeSpeed)
	}
	return allTreeSpeed
}

func (i *InfinibandInfo) GetPCIETreeWidth(IBDev string) []PCIETreeWidthInfo {
	bdf := i.GetBDF(IBDev)[0]
	devicePath := filepath.Join(PCIPath, bdf)
	cmd := exec.Command("readlink", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	bdfRegexPattern := `\b[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]\b`
	re := regexp.MustCompile(bdfRegexPattern)
	bdfs := re.FindAllString(string(output), -1)
	allTreeWidth := make([]PCIETreeWidthInfo, 0, len(bdfs))

	for _, bdf := range bdfs {
		var perTreeWidth PCIETreeWidthInfo
		width := GetFileCnt(filepath.Join(PCIPath, bdf, "current_link_width"))
		logrus.WithField("component", "infiniband").Infof("get the pcie tree width, ib:%s bdf:%s width:%s", IBDev, bdf, width[0])
		perTreeWidth.BDF = bdf
		perTreeWidth.Width = width[0]
		allTreeWidth = append(allTreeWidth, perTreeWidth)
	}
	return allTreeWidth
}

func (i *InfinibandInfo) GetPCIECLinkSpeed(IBDev string) []string {
	return i.GetSysCnt(IBDev, "device/current_link_speed")
}

func (i *InfinibandInfo) GetPCIECLinkWidth(IBDev string) []string {
	return i.GetSysCnt(IBDev, "device/current_link_width")
}

func (i *InfinibandInfo) GetSysCnt(IBDev string, DstPath string) []string {
	var allCnt []string
	DesPath := path.Join(IBSYSPathPre, IBDev, DstPath)
	Cnt := GetFileCnt(DesPath)
	allCnt = append(allCnt, Cnt...)
	return allCnt
}

func (i *InfinibandInfo) GetDeviceID(IBDev string) []string {
	return i.GetSysCnt(IBDev, "device/device")
}

func (i *InfinibandInfo) GetBDF(IBDev string) []string {
	ueventInfo := i.GetSysCnt(IBDev, "device/uevent")
	if len(ueventInfo) == 0 {
		return nil
	}

	var BDF string
	for j := 0; j < len(ueventInfo); j++ {
		if strings.Contains(ueventInfo[j], "PCI_SLOT_NAME") {
			BDF = strings.Split(ueventInfo[j], "=")[1]
		}
	}
	return []string{BDF}

}

func (i *InfinibandInfo) GetNumaNode(IBDev string) []string {
	BDF := i.GetBDF(IBDev)
	DesPath := path.Join(PCIPath, BDF[0], "numa_node")
	numaNode := GetFileCnt(DesPath)
	return numaNode
}

func (i *InfinibandInfo) GetCPUList(IBDev string) []string {
	BDF := i.GetBDF(IBDev)
	DesPath := path.Join(PCIPath, BDF[0], "local_cpulist")
	CPUList := GetFileCnt(DesPath)
	return CPUList
}

func (i *InfinibandInfo) GetOFEDInfo(ctx context.Context) string {
	var ver string
	// cmd := exec.Command("ofed_info", "-s")
	output, err := utils.ExecCommand(ctx, "ofed_info", "-s")
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Fail to run the cmd: ofed_info -s, err:%v", err)
	}
	outputStr := string(output)
	lines := strings.Split(outputStr, ":")
	ver = lines[0]
	if len(ver) == 0 {
		ver = "Not Get"
	}

	return ver
}

func (i *InfinibandInfo) GetSystemGUID(IBDev string) []string {
	return i.GetSysCnt(IBDev, "sys_image_guid")
}

func (i *InfinibandInfo) GetNodeGUID(IBDev string) []string {
	return i.GetSysCnt(IBDev, "node_guid")
}

func (i *InfinibandInfo) GetKernelModule() []string {
	preInstallModule := []string{
		"rdma_ucm",
		"rdma_cm",
		"ib_ipoib",
		"mlx5_core",
		"mlx5_ib",
		"ib_uverbs",
		"ib_umad",
		"ib_cm",
		"ib_core",
		"mlxfw"}
	if utils.IsNvidiaGPUExist() {
		preInstallModule = append(preInstallModule, "nvidia_peermem")
	}

	var installedModule []string
	for _, module := range preInstallModule {
		installed := IsModuleLoaded(module)
		if installed {
			installedModule = append(installedModule, module)
		} else {
			logrus.WithField("component", "infiniband").Errorf("Fail to load the kernel module %s", module)
		}
	}

	return installedModule
}

func NewIBCollector() (*InfinibandInfo, error) {
	var IBInfo InfinibandInfo
	info, err := IBInfo.Collect(context.Background())
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to collect Infiniband info: %v", err)
		return nil, fmt.Errorf("failed to collect Infiniband info: %w", err)
	}

	IBInfoPtr, ok := info.(*InfinibandInfo)
	if !ok {
		logrus.WithField("component", "infiniband").Errorf("Failed to cast collected info to InfinibandInfo type")
		return nil, fmt.Errorf("failed to cast collected info to InfinibandInfo type")
	}

	return IBInfoPtr, nil
}

func (i *InfinibandInfo) Collect(ctx context.Context) (common.Info, error) {
	var IBInfo InfinibandInfo
	var IBSWInfo IBSoftWareInfo

	IBSWInfo.OFEDVer = i.GetOFEDInfo(ctx)
	IBSWInfo.KernelModule = i.GetKernelModule()

	IBInfo.IBDevs = IBInfo.GetIBdevs()
	IBInfo.HCAPCINum = len(IBInfo.IBDevs)
	IBHWInfo := make([]IBHardWareInfo, 0, len(IBInfo.IBDevs))
	IBCounters := make(map[string]map[string]uint64, len(IBInfo.IBDevs))
	for IBDev := range IBInfo.IBDevs {
		var perIBHWInfo IBHardWareInfo
		perIBHWInfo.IBDev = IBDev
		perIBHWInfo.NetOperstate = i.GetNetOperstate(IBDev)
		if len(i.GetHCAType(IBDev)) >= 1 {
			perIBHWInfo.HCAType = i.GetHCAType(IBDev)[0]
		}
		if len(i.GetPhyStat(IBDev)) >= 1 {
			perIBHWInfo.PhyState = i.GetPhyStat(IBDev)[0]
		}
		if len(i.GetIBStat(IBDev)) >= 1 {
			perIBHWInfo.PortState = i.GetIBStat(IBDev)[0]
		}
		if len(i.GetLinkLayer(IBDev)) >= 1 {
			perIBHWInfo.LinkLayer = i.GetLinkLayer(IBDev)[0]
		}
		if len(i.GetPortSpeed(IBDev)) >= 1 {
			perIBHWInfo.PortSpeed = i.GetPortSpeed(IBDev)[0]
		}
		if len(i.GetBoardID(IBDev)) >= 1 {
			perIBHWInfo.BoardID = i.GetBoardID(IBDev)[0]
		}
		if len(i.GetDeviceID(IBDev)) >= 1 {
			perIBHWInfo.DeviceID = i.GetDeviceID(IBDev)[0]
		}
		if len(i.GetBDF(IBDev)) >= 1 {
			perIBHWInfo.PCIEBDF = i.GetBDF(IBDev)[0]
		}
		if len(i.GetPCIEMRR(ctx, IBDev)) >= 1 {
			perIBHWInfo.PCIEMRR = i.GetPCIEMRR(ctx, IBDev)[0]
		}
		if len(i.GetPCIECLinkSpeed(IBDev)) >= 1 {
			perIBHWInfo.PCIESpeed = i.GetPCIECLinkSpeed(IBDev)[0]
		}
		if len(i.GetPCIECLinkWidth(IBDev)) >= 1 {
			perIBHWInfo.PCIEWidth = i.GetPCIECLinkWidth(IBDev)[0]
		}
		if len(i.GetNumaNode(IBDev)) >= 1 {
			perIBHWInfo.NumaNode = i.GetNumaNode(IBDev)[0]
		}
		if len(i.GetCPUList(IBDev)) >= 1 {
			perIBHWInfo.CPULists = i.GetCPUList(IBDev)[0]
		}
		if len(i.GetFWVer(IBDev)) >= 1 {
			perIBHWInfo.FWVer = i.GetFWVer(IBDev)[0]
		}
		if len(i.GetIBdev2NetDev(IBDev)) >= 1 {
			perIBHWInfo.NetDev = i.GetIBdev2NetDev(IBDev)[0]
		}
		if len(i.GetSystemGUID(IBDev)) >= 1 {
			perIBHWInfo.SystemGUID = i.GetSystemGUID(IBDev)[0]
		}
		if len(i.GetNodeGUID(IBDev)) >= 1 {
			perIBHWInfo.NodeGUID = i.GetNodeGUID(IBDev)[0]
		}
		// perIBHWInfo.PCIETreeSpeed = i.GetPCIETreeSpeed(IBDev)
		// perIBHWInfo.PCIETreeWidth = i.GetPCIETreeWidth(IBDev)
		perIBHWInfo.VPD = i.GetVPD(IBDev)
		IBHWInfo = append(IBHWInfo, perIBHWInfo)
		IBCounters[IBDev] = i.GetIBCounters(IBDev)

	}
	IBInfo.IBHardWareInfo = IBHWInfo
	IBInfo.IBSoftWareInfo = IBSWInfo
	IBInfo.IBCounters = IBCounters
	IBInfo.Time = time.Now()
	return &IBInfo, nil
}

func IsModuleLoaded(moduleName string) bool {
	file, err := os.Open("/proc/modules")
	if err != nil {
		fmt.Printf("Unable to open the /proc/modules file: %v\n", err)
		return false
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Error closing file: %v\n", closeErr)
		}
	}()

	return checkModuleInFile(moduleName, file)
}

func checkModuleInFile(moduleName string, file *os.File) bool {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == moduleName {
			return true
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("An error occurred while reading the file: %v\n", err)
	}

	return false
}

func GetFileCnt(path string) []string {
	fileInfo, err := os.Stat(path)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Invalid Path: %v", err)
		return nil
	}

	var results []string
	if fileInfo.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read directory: %v", err)
			return nil
		}

		for _, file := range files {
			results = append(results, file.Name())
		}
	} else {
		results = readFileContent(path)
	}
	return results
}

func readFileContent(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to open file: %v", err)
		return nil
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Error closing file: %v\n", closeErr)
		}
	}()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		logrus.WithField("component", "infiniband").Errorf("Error while reading file: %v", err)
	}
	return lines
}
