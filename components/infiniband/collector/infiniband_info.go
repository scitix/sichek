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
	"strings"
	"time"

	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

var (
	IBSYSPathPre string = "/sys/class/infiniband/"
	IBDevPathPre string = "/dev/infiniband/"
)

type InfinibandInfo struct {
	IBDevs         []string         `json:"ib_dev"`
	IBHardWareInfo []IBHardWareInfo `json:"ib_hardware_info"`
	IBSoftWareInfo IBSoftWareInfo   `json:"ib_software_info"`
	Time           time.Time        `json:"time"`
}

type IBHardWareInfo struct {
	IBDev         string              `json:"IBdev"`
	NetDev        string              `json:"net_dev"`
	HCAType       string              `json:"hca_type"`
	PhyStat       string              `json:"phy_stat"`
	PortStat      string              `json:"port_stat"`
	LinkLayer     string              `json:"link_layer"`
	PortSpeed     string              `json:"port_speed"`
	BoardID       string              `json:"bodard_id"`
	DeviceID      string              `json:"device_id"`
	PCIEBDF       string              `json:"pcie_bdf"`
	PCIESpeed     string              `json:"pcie_speed"`
	PCIEWidth     string              `json:"pcie_width"`
	PCIEMRR       string              `json:"pcie_mrr"`
	PCIETreeSpeed []PCIETreeSpeedInfo `json:"pcie_tree_speed"`
	PCIETreeWidth []PCIETreeWidthInfo `json:"pcie_tree_width"`
	Slot          string              `json:"slot"`
	NumaNode      string              `json:"numa_node"`
	CPULists      string              `json:"cpu_lists"`
	FWVer         string              `json:"fw_ver"`
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
	OFEDVer      string   `json:"driver_ver"`
	KernelModule []string `json:"kernel_module"`
}

func (i *InfinibandInfo) JSON() (string, error) {
	data, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (i *InfinibandInfo) GetIBdevs() []string {
	return GetFileCnt(IBSYSPathPre)
}

func (i *InfinibandInfo) GetIBdev2NetDev(IBDev string) []string {
	return i.GetSysCnt(IBDev, "device/net")
}

func (i *InfinibandInfo) GetHCAType(IBDev string) []string {
	return i.GetSysCnt(IBDev, "hca_type")
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

func (i *InfinibandInfo) GetPCIEMRR(IBDev string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	devicePath := filepath.Join("/sys/bus/pci/devices", bdf)
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
		speed := GetFileCnt(filepath.Join("/sys/bus/pci/devices", bdf, "current_link_speed"))
		logrus.WithField("component", "infiniband").Infof("get the pcie tree speed, ib:%s bdf:%s speed:%s", IBDev, bdf, speed[0])
		perTreeSpeed.BDF = bdf
		perTreeSpeed.Speed = speed[0]
		allTreeSpeed = append(allTreeSpeed, perTreeSpeed)
	}
	return allTreeSpeed
}

func (i *InfinibandInfo) GetPCIETreeWidth(IBDev string) []PCIETreeWidthInfo {
	bdf := i.GetBDF(IBDev)[0]
	devicePath := filepath.Join("/sys/bus/pci/devices", bdf)
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
		width := GetFileCnt(filepath.Join("/sys/bus/pci/devices", bdf, "current_link_width"))
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
	DesPath := path.Join("/sys/bus/pci/devices", BDF[0], "numa_node")
	numaNode := GetFileCnt(DesPath)
	return numaNode
}

func (i *InfinibandInfo) GetCPUList(IBDev string) []string {
	BDF := i.GetBDF(IBDev)
	DesPath := path.Join("/sys/bus/pci/devices", BDF[0], "local_cpulist")
	CPUList := GetFileCnt(DesPath)
	return CPUList
}

func (i *InfinibandInfo) GetOFEDInfo() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ver string
	// cmd := exec.Command("ofed_info", "-s")
	output, err := utils.ExecCommand(ctx, "ofed_info", "-s")
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Fail to run the cmd: ofer_info -s")
	}
	outputStr := string(output)
	lines := strings.Split(outputStr, ":")
	ver = lines[0]
	if len(ver) == 0 {
		ver = "Not Get"
	}

	return ver
}

func (i *InfinibandInfo) GetKernelModule() []string {
	var preInstallModule []string
	preInstallModule = append(preInstallModule,
		"rdma_ucm",
		"rdma_cm",
		"ib_ipoib",
		"mlx5_core",
		"mlx5_ib",
		"ib_uverbs",
		"ib_umad",
		"ib_cm",
		"ib_core",
		"mlxfw",
		"nvidia_peermem")

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

func (i *InfinibandInfo) GetSpec() *config.InfinibandHCASpec {
	var IBSpec *config.InfinibandHCASpec
	ibSpec, err := IBSpec.GetHCASpec()
	if err != nil {
		logrus.WithField("component", "infiniband").Error("fail to get hca spec ", err)
	}

	IBSpecJSON, err := ibSpec.JSON()
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("fail to Marshal ibSpec %v", err)
	}
	logrus.WithField("component", "infiniband").Infof("load ib spec %v ", IBSpecJSON)
	return ibSpec
}

func (i *InfinibandInfo) GetIBInfo() *InfinibandInfo {
	var IBInfo InfinibandInfo
	var IBSWInfo IBSoftWareInfo

	IBSWInfo.OFEDVer = i.GetOFEDInfo()
	IBSWInfo.KernelModule = i.GetKernelModule()

	IBInfo.IBDevs = IBInfo.GetIBdevs()
	IBHWInfo := make([]IBHardWareInfo, 0, len(IBInfo.IBDevs))
	for _, IBDev := range IBInfo.IBDevs {
		var perIBHWInfo IBHardWareInfo
		perIBHWInfo.IBDev = IBDev
		perIBHWInfo.HCAType = i.GetHCAType(IBDev)[0]
		perIBHWInfo.PhyStat = i.GetPhyStat(IBDev)[0]
		perIBHWInfo.PortStat = i.GetIBStat(IBDev)[0]
		perIBHWInfo.LinkLayer = i.GetLinkLayer(IBDev)[0]
		perIBHWInfo.PortSpeed = i.GetPortSpeed(IBDev)[0]
		perIBHWInfo.BoardID = i.GetBoardID(IBDev)[0]
		perIBHWInfo.DeviceID = i.GetDeviceID(IBDev)[0]
		perIBHWInfo.PCIEBDF = i.GetBDF(IBDev)[0]
		perIBHWInfo.PCIEMRR = i.GetPCIEMRR(IBDev)[0]
		perIBHWInfo.PCIESpeed = i.GetPCIECLinkSpeed(IBDev)[0]
		perIBHWInfo.PCIEWidth = i.GetPCIECLinkWidth(IBDev)[0]
		perIBHWInfo.PCIETreeSpeed = i.GetPCIETreeSpeed(IBDev)
		perIBHWInfo.PCIETreeWidth = i.GetPCIETreeWidth(IBDev)
		perIBHWInfo.NumaNode = i.GetNumaNode(IBDev)[0]
		perIBHWInfo.CPULists = i.GetCPUList(IBDev)[0]
		perIBHWInfo.FWVer = i.GetFWVer(IBDev)[0]
		perIBHWInfo.NetDev = i.GetIBdev2NetDev(IBDev)[0]
		IBHWInfo = append(IBHWInfo, perIBHWInfo)
	}
	IBInfo.IBHardWareInfo = IBHWInfo
	IBInfo.IBSoftWareInfo = IBSWInfo
	IBInfo.Time = time.Now()
	return &IBInfo
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
