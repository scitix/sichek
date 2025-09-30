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
	"math"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

var (
	IBSYSPathPre    = "/sys/class/infiniband/"
	PCIPath         = "/sys/bus/pci/devices/"
	gatewayCacheTTL = 5 * time.Minute
)

type InfinibandInfo struct {
	HCAPCINum      int                           `json:"hca_pci_num"`
	IBPFDevs       map[string]string             `json:"ib_dev"`
	IBPCIDevs      map[string]string             `json:"hca_pci_dev"`
	IBHardWareInfo map[string]IBHardWareInfo     `json:"ib_hardware_info"`
	IBSoftWareInfo IBSoftWareInfo                `json:"ib_software_info"`
	PCIETreeInfo   map[string]PCIETreeInfo       `json:"pcie_tree_info"`
	IBCounters     map[string]map[string]uint64  `json:"ib_counters"`
	IBNicRole      string                        `json:"ib_nic_role"`
	Time           time.Time                     `json:"time"`
	GWCache        map[string]*gatewayCacheEntry `json:"gateway_cache"`
	mu             sync.RWMutex
}

type gatewayCacheEntry struct {
	GatewayIP string    `json:"gateway_ip"`
	Err       error     `json:"error"`
	Timestamp time.Time `json:"timestamp"`
}

type IBHardWareInfo struct {
	IBDev            string `json:"IBdev"`
	NetDev           string `json:"net_dev"`
	HCAType          string `json:"hca_type"`
	SystemGUID       string `json:"system_guid"`
	NodeGUID         string `json:"node_guid"`
	PFGW             string `json:"pf_gw"`
	VFSpec           string `json:"vf_spec"`
	VFNum            string `json:"vf_num"`
	PhyState         string `json:"phy_state"`
	PortState        string `json:"port_state"`
	LinkLayer        string `json:"link_layer"`
	NetOperstate     string `json:"net_operstate"`
	PortSpeed        string `json:"port_speed"`
	PortSpeedState   string `json:"port_speed_state"`
	BoardID          string `json:"board_id"`
	DeviceID         string `json:"device_id"`
	PCIEBDF          string `json:"pcie_bdf"`
	PCIESpeed        string `json:"pcie_speed"`
	PCIESpeedState   string `json:"pcie_speed_state"`
	PCIEWidth        string `json:"pcie_width"`
	PCIEWidthState   string `json:"pcie_width_state"`
	PCIETreeSpeedMin string `json:"pcie_tree_speed"`
	PCIETreeWidthMin string `json:"pcie_tree_width"`
	PCIEMRR          string `json:"pcie_mrr"`
	Slot             string `json:"slot"`
	NumaNode         string `json:"numa_node"`
	CPULists         string `json:"cpu_lists"`
	FWVer            string `json:"fw_ver"`
	VPD              string `json:"vpd"`
	OFEDVer          string `json:"ofed_ver"` // compatible with IB Spec Requirement
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

func (i *InfinibandInfo) GetPFDevs(IBDevs []string) []string {
	PFDevs := make([]string, 0)
	for _, IBDev := range IBDevs {
		if strings.Contains(IBDev, "mezz") {
			continue
		}
		vfPath := path.Join(IBSYSPathPre, IBDev, "device", "physfn")
		if _, err := os.Stat(vfPath); err == nil {
			continue // Skip virtual functions
		} else {
			PFDevs = append(PFDevs, IBDev)
		}
	}
	return PFDevs
}

// get from /sys/class/infiniband/
func (i *InfinibandInfo) GetIBPFdevs() map[string]string {
	allIBDevs := GetFileCnt(IBSYSPathPre)
	PFDevs := i.GetPFDevs(allIBDevs)

	IBPFDevs := make(map[string]string)
	for _, IBDev := range PFDevs {
		if strings.Contains(IBDev, "bond") {
			continue
		}
		netPath := filepath.Join(IBSYSPathPre, IBDev, "device/net")
		if _, err := os.Stat(netPath); os.IsNotExist(err) {
			logrus.WithField("component", "infiniband").Warnf("No net directory found for IB device %s, skipping", IBDev)
			continue
		}
		var ibNetDev string
		if len(GetFileCnt(netPath)) == 0 {
			logrus.WithField("component", "infiniband").Warnf("No network interfaces found for IB device %s, skipping", IBDev)
			ibNetDev = ""
		} else {
			ibNetDev = GetFileCnt(netPath)[0]
		}
		IBPFDevs[IBDev] = ibNetDev
	}
	logrus.WithField("component", "infiniband").Infof("get the IB and net map: %v", IBPFDevs)

	return IBPFDevs
}

func (i *InfinibandInfo) FindIBPCIDevices(targetVendorID string, targetDeviceIDs []string) (map[string]string, error) {
	log := logrus.WithField("component", "pci-scanner")

	if len(targetDeviceIDs) == 0 {
		log.Info("Target device ID list is empty, no devices can be matched.")
		return make(map[string]string), nil
	}

	if _, err := os.Stat(PCIPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("pci devices directory not found at %s: %w", PCIPath, err)
	}

	entries, err := os.ReadDir(PCIPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pci devices directory %s: %w", PCIPath, err)
	}

	log.Infof("Scanning PCI devices in %s for vendor ID %s and device IDs %v", PCIPath, targetVendorID, targetDeviceIDs)

	foundDevices := make(map[string]string)

	for _, entry := range entries {
		pciAddr := entry.Name()
		deviceDir := filepath.Join(PCIPath, pciAddr)

		// 读取并比较厂商ID
		vendorBytes, err := os.ReadFile(filepath.Join(deviceDir, "vendor"))
		if err != nil {
			log.Warnf("Could not read vendor file for %s, skipping. Error: %v", pciAddr, err)
			continue
		}
		currentVendorID := strings.TrimSpace(string(vendorBytes))

		// 如果厂商ID不匹配，直接跳过此设备
		if currentVendorID != targetVendorID {
			continue
		}

		// 厂商ID匹配，接着读取设备ID
		deviceBytes, err := os.ReadFile(filepath.Join(deviceDir, "device"))
		if err != nil {
			log.Warnf("Could not read device file for %s, skipping. Error: %v", pciAddr, err)
			continue
		}
		currentDeviceID := strings.TrimSpace(string(deviceBytes))

		// 检查设备ID是否在目标列表中

		if slices.Contains(targetDeviceIDs, currentDeviceID) {
			log.Infof("Found matching device: %s with vendor=%s, device=%s ", pciAddr, currentVendorID, currentDeviceID)
			foundDevices[pciAddr] = fmt.Sprintf("%s:%s", currentVendorID, currentDeviceID)
		}
	}

	log.Infof("Finished PCI scan. Found %d matching devices.", len(foundDevices))
	return foundDevices, nil
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

func (i *InfinibandInfo) GetIBDeVLinklayer(IBDev string) string {
	var linkLayer string
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
	operstatePath := filepath.Join(netPath, netDir, "type")
	data, err := os.ReadFile(operstatePath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to read %s: %v", operstatePath, err)
		return ""
	}
	if strings.Compare(strings.TrimSpace(string(data)), "1") == 0 {
		linkLayer = "Ethernet"
	}
	if strings.Compare(strings.TrimSpace(string(data)), "32") == 0 {
		linkLayer = "InfiniBand"
	}

	return linkLayer
}

// _findGatewayWithNetlink 是一个私有的、无缓存的函数，负责执行实际的Netlink查询。
func (i *InfinibandInfo) _findGatewayWithNetlink(ifaceName string) (string, error) {
	// 通过接口名获取接口的Link对象
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("netlink: failed to find interface '%s': %w", ifaceName, err)
	}

	// 只查找与该接口关联的IPv4路由
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return "", fmt.Errorf("netlink: failed to list routes for '%s': %w", ifaceName, err)
	}

	// 遍历路由表，寻找网关
	var fallbackGateway net.IP
	for _, route := range routes {
		// 优先返回默认路由 (目标地址Dst为nil，且网关Gw不为nil)
		if route.Dst == nil && route.Gw != nil {
			logrus.WithField("component", "infiniband").Infof("Found default gateway: %s for interface %s", route.Gw.String(), ifaceName)
			return route.Gw.String(), nil
		}

		// 如果没有默认路由，则记录第一个找到的非默认网关作为备选方案。
		// 这在某些复杂的网络配置中很有用。
		if fallbackGateway == nil && route.Gw != nil {
			fallbackGateway = route.Gw
		}
	}

	// 如果循环结束后找到了备选网关，则返回它
	if fallbackGateway != nil {
		logrus.WithField("component", "infiniband").Infof("Using fallback get gateway: %s for interface %s", fallbackGateway.String(), ifaceName)
		return fallbackGateway.String(), nil
	}

	// 如果遍历完所有路由都没有找到任何网关
	return "", fmt.Errorf("netlink: no gateway found for interface '%s'", ifaceName)
}

func (i *InfinibandInfo) FindGateway(ifaceName string) (string, error) {
	// --- 1. 快速路径: 使用读锁检查缓存 ---
	i.mu.RLock()
	entry, exists := i.GWCache[ifaceName]
	if exists && time.Since(entry.Timestamp) < gatewayCacheTTL {
		logrus.WithField("component", "infiniband").Infof("Gateway cache hit for interface %s: %s", ifaceName, entry.GatewayIP)
		i.mu.RUnlock()
		return entry.GatewayIP, entry.Err // 直接返回缓存的结果
	}
	i.mu.RUnlock()

	// --- 2. 慢速路径: 使用写锁执行查询并更新缓存 ---
	i.mu.Lock()
	defer i.mu.Unlock()

	// --- 双重检查锁定: 在我们等待写锁时，可能已有其他goroutine完成了刷新 ---
	entry, exists = i.GWCache[ifaceName]
	if exists && time.Since(entry.Timestamp) < gatewayCacheTTL {
		logrus.WithField("component", "infiniband").Infof("Gateway cache hit after lock for interface %s.", ifaceName)
		return entry.GatewayIP, entry.Err
	}

	// --- 缓存未命中或已过期，执行真正的查询 ---
	gateway, err := i._findGatewayWithNetlink(ifaceName)

	// --- 将新结果写入缓存 ---
	i.GWCache[ifaceName] = &gatewayCacheEntry{
		GatewayIP: gateway,
		Err:       err,
		Timestamp: time.Now(),
	}

	return gateway, err
}

func (i *InfinibandInfo) GetPFGW(IBDev string) string {
	linkLayer := i.GetIBDeVLinklayer(IBDev)

	switch linkLayer {
	case "InfiniBand":
		logrus.WithField("component", "infiniband").Debugf("No gateway for InfiniBand device: %s", IBDev)
		return ""

	case "Ethernet":
		netDevs := i.GetIBdev2NetDev(IBDev)
		if len(netDevs) == 0 {
			logrus.WithField("component", "infiniband").Errorf("No network device found for IB device: %s[%s], ", IBDev, netDevs)
			return ""
		}
		ifaceName := netDevs[0]

		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to find interface %s[%s]: %v", IBDev, ifaceName, err)
			return ""
		}

		if iface.Flags&net.FlagUp == 0 {
			logrus.WithField("component", "infiniband").Warnf("Interface %s is down, cannot find gateway", ifaceName)
			return ""
		}

		gatewayIP, err := i.FindGateway(ifaceName)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to find gateway for interface %s: %v", ifaceName, err)
			return ""
		}
		return gatewayIP

	default:
		logrus.WithField("component", "infiniband").Errorf("Unsupported link layer type: %s for IB device: %s", linkLayer, IBDev)
		return ""
	}
}

func (i *InfinibandInfo) GetVFSpec(IBDev string) string {
	netPath := filepath.Join(IBSYSPathPre, IBDev, "device", "sriov_totalvfs")
	data, err := os.ReadFile(netPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to read net directory %s: %v", netPath, err)
		return ""
	}

	return strings.TrimSpace(string(data))
}

func (i *InfinibandInfo) GetVFNum(IBDev string) string {
	var vfNum string
	bdf := i.GetBDF(IBDev)[0]

	// skip secondary port
	if strings.HasSuffix(bdf, ".1") {
		return "0"
	}

	deviceName := i.GetIBdev2NetDev(IBDev)[0]

	cmd := exec.Command("ip", "link", "show", "dev", deviceName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ""
	}

	if err := cmd.Start(); err != nil {
		return ""
	}

	count := 0
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "vf") && !strings.Contains(line, "00:00:00:00:00:00") {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return ""
	}

	if err := cmd.Wait(); err != nil {
		return ""
	}
	vfNum = strconv.Itoa(count)

	return vfNum
}

func (i *InfinibandInfo) Name() string {
	return "IBcollector"
}

func (i *InfinibandInfo) GetNICRole() string {
	var nodeState string

	cmd := exec.Command("rdma", "system")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "ErrNode"
	}
	outputStr := string(output)
	if strings.Contains(outputStr, "exclusive") {
		nodeState = "sriovNode"
	}

	if strings.Contains(outputStr, "share") {
		nodeState = "macvlanNode"
	}

	return nodeState
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
			Counters, err = i.GetIBCounter(IBDev, ct)
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
		return string(bytes.TrimSpace(data))
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

func (i *InfinibandInfo) GetPCIETreeMin(IBDev, linkType string) string {
	bdfList := i.GetBDF(IBDev)
	if len(bdfList) == 0 {
		logrus.WithField("component", "infiniband").Warnf("Could not get BDF for IB device %s", IBDev)
		return ""
	}
	// bdf 是终端设备自身的 BDF 地址
	bdf := bdfList[0]

	devicePath := filepath.Join(PCIPath, bdf)
	linkPath, err := os.Readlink(devicePath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to resolve symlink for %s: %v", devicePath, err)
		return ""
	}

	bdfRegexPattern := `\b[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]\b`
	re := regexp.MustCompile(bdfRegexPattern)

	allBdfsInPath := re.FindAllString(string(linkPath), -1)

	// 过滤掉设备自身的BDF。
	var upstreamBdfs []string
	for _, foundBdf := range allBdfsInPath {
		if foundBdf != bdf {
			upstreamBdfs = append(upstreamBdfs, foundBdf)
		}
	}

	if len(upstreamBdfs) == 0 {
		// 如果没有上游设备（例如设备直连CPU），这很正常，直接返回即可。
		logrus.WithField("component", "infiniband").Infof("No upstream PCIe devices found in path for %s, skipping check.", bdf)
		return ""
	}

	logrus.WithField("component", "infiniband").Infof("Checking upstream devices for %s: %v", bdf, upstreamBdfs)

	var minNumericString string
	minNumericValue := math.MaxFloat64

	// 现在，我们只遍历上游设备的BDF列表
	for _, currentBdf := range upstreamBdfs {
		propertyFilePath := filepath.Join(PCIPath, currentBdf, linkType)

		propertyContents := GetFileCnt(propertyFilePath)
		if len(propertyContents) == 0 {
			logrus.WithField("component", "infiniband").Debugf("Property file '%s' is empty or unreadable for BDF %s, skipping.", linkType, currentBdf)
			continue
		}

		currentPropertyString := strings.TrimSpace(propertyContents[0])
		parts := strings.Fields(currentPropertyString)
		if len(parts) == 0 {
			logrus.WithField("component", "infiniband").Warnf("Malformed property string '%s' for BDF %s", currentPropertyString, currentBdf)
			continue
		}
		numericStringPart := parts[0]

		currentNumericValue, err := strconv.ParseFloat(numericStringPart, 64)
		if err != nil {
			logrus.WithField("component", "infiniband").Warnf("Could not parse numeric value from '%s' in file %s", numericStringPart, propertyFilePath)
			continue
		}

		if currentNumericValue < minNumericValue {
			minNumericValue = currentNumericValue
			minNumericString = numericStringPart
			logrus.WithField("component", "infiniband").Infof(
				"Found new upstream minimum for %s (%s): %s (full value: '%s', at BDF: %s)",
				IBDev, linkType, minNumericString, currentPropertyString, currentBdf,
			)
		}
	}

	if minNumericString == "" {
		logrus.WithField("component", "infiniband").Warnf("Could not determine a minimum value for property '%s' on upstream path of device %s", linkType, IBDev)
	}

	return minNumericString
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
func (i *InfinibandInfo) Collect(ctx context.Context) (common.Info, error) {
	i.IBPCIDevs, _ = i.FindIBPCIDevices("0x15b3", []string{
		"0x101b", // MT28908 Family [ConnectX-6]
		"0x1021", // CMT2910 Family [ConnectX-7]
		"0x1023", // CX8 Family [ConnectX-8]
	})
	i.IBPFDevs = i.GetIBPFdevs()
	i.HCAPCINum = len(i.IBPFDevs)
	for IBDev := range i.IBPFDevs {
		i.IBCounters[IBDev] = i.GetIBCounters(IBDev)
		perIBHWInfo := i.IBHardWareInfo[IBDev]
		perIBHWInfo.NetOperstate = i.GetNetOperstate(IBDev)
		if i.IBNicRole == "sriovNode" {
			perIBHWInfo.VFNum = i.GetVFNum(IBDev)
			perIBHWInfo.VFSpec = i.GetVFSpec(IBDev)
		}
		perIBHWInfo.PFGW = i.GetPFGW(IBDev)
		phyState := i.GetPhyStat(IBDev)
		if len(phyState) >= 1 {
			perIBHWInfo.PhyState = phyState[0]
		}
		portState := i.GetIBStat(IBDev)
		if len(portState) >= 1 {
			perIBHWInfo.PortState = portState[0]
		}
		pcieMrr := i.GetPCIEMRR(ctx, IBDev)
		if len(pcieMrr) >= 1 {
			perIBHWInfo.PCIEMRR = pcieMrr[0]
		}
		portSpeed := i.GetPortSpeed(IBDev)
		if len(portSpeed) >= 1 {
			perIBHWInfo.PortSpeed = portSpeed[0]
		}
		pcieSpeed := i.GetPCIECLinkSpeed(IBDev)
		if len(pcieSpeed) >= 1 {
			perIBHWInfo.PCIESpeed = pcieSpeed[0]
		}
		pcieWidth := i.GetPCIECLinkWidth(IBDev)
		if len(pcieWidth) >= 1 {
			perIBHWInfo.PCIEWidth = pcieWidth[0]
		}
		netDevs := i.GetIBdev2NetDev(IBDev)
		if len(netDevs) >= 1 {
			perIBHWInfo.NetDev = netDevs[0]
		}

		perIBHWInfo.PCIETreeSpeedMin = i.GetPCIETreeMin(IBDev, "current_link_speed")
		perIBHWInfo.PCIETreeWidthMin = i.GetPCIETreeMin(IBDev, "current_link_width")

		i.IBHardWareInfo[IBDev] = perIBHWInfo
	}
	i.Time = time.Now()

	return i, nil
}

func (i *InfinibandInfo) GetIBCounter(IBDev string, counterType string) (map[string]uint64, error) {
	Counters := make(map[string]uint64, 0)
	counterPath := path.Join(IBSYSPathPre, IBDev, "ports/1", counterType)
	ibCounterName, err := listFiles(counterPath)
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

func NewIBCollector(ctx context.Context) (*InfinibandInfo, error) {
	i := &InfinibandInfo{
		GWCache:        make(map[string]*gatewayCacheEntry),
		IBHardWareInfo: make(map[string]IBHardWareInfo),
		IBSoftWareInfo: IBSoftWareInfo{},
		PCIETreeInfo:   make(map[string]PCIETreeInfo),
		IBPFDevs:       make(map[string]string),
		mu:             sync.RWMutex{},
		// Initialize counters map with IB devices
		IBCounters: make(map[string]map[string]uint64),
	}
	i.IBPCIDevs, _ = i.FindIBPCIDevices("0x15b3", []string{
		"0x1021", // ConnectX-7
	})
	i.IBPFDevs = i.GetIBPFdevs()
	i.HCAPCINum = len(i.IBPFDevs)
	i.IBNicRole = i.GetNICRole()

	i.IBSoftWareInfo.OFEDVer = i.GetOFEDInfo(ctx)
	i.IBSoftWareInfo.KernelModule = i.GetKernelModule()

	for IBDev := range i.IBPFDevs {
		var perIBHWInfo IBHardWareInfo
		perIBHWInfo.IBDev = IBDev
		perIBHWInfo.NetOperstate = i.GetNetOperstate(IBDev)
		if i.IBNicRole == "sriovNode" {
			perIBHWInfo.VFNum = i.GetVFNum(IBDev)
			perIBHWInfo.VFSpec = i.GetVFSpec(IBDev)
		}
		perIBHWInfo.PFGW = i.GetPFGW(IBDev)
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
		perIBHWInfo.PCIETreeSpeedMin = i.GetPCIETreeMin(IBDev, "current_link_speed")
		perIBHWInfo.PCIETreeWidthMin = i.GetPCIETreeMin(IBDev, "current_link_width")

		perIBHWInfo.VPD = i.GetVPD(IBDev)
		i.IBHardWareInfo[IBDev] = perIBHWInfo
		i.IBCounters[IBDev] = i.GetIBCounters(IBDev)
	}

	i.Time = time.Now()
	return i, nil
}

func listFiles(dir string) ([]string, error) {
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
