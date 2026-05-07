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
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type IBHardWareInfo struct {
	IBDev            string `json:"IBdev" yaml:"IBdev"`
	Port             int    `json:"port,omitempty" yaml:"port,omitempty"`
	NetDev           string `json:"net_dev" yaml:"net_dev"`
	HCAType          string `json:"hca_type" yaml:"hca_type"`
	SystemGUID       string `json:"system_guid" yaml:"system_guid"`
	NodeGUID         string `json:"node_guid" yaml:"node_guid"`
	PFGW             string `json:"pf_gw" yaml:"pf_gw"`
	VFSpec           string `json:"vf_spec" yaml:"vf_spec"`
	VFNum            string `json:"vf_num" yaml:"vf_num"`
	PhyState         string `json:"phy_state" yaml:"phy_state"`
	PortState        string `json:"port_state" yaml:"port_state"`
	LinkLayer        string `json:"link_layer" yaml:"link_layer"`
	NetOperstate     string `json:"net_operstate" yaml:"net_operstate"`
	PortSpeed        string `json:"port_speed" yaml:"port_speed"`
	PortSpeedState   string `json:"port_speed_state" yaml:"port_speed_state"`
	BoardID          string `json:"board_id" yaml:"board_id"`
	DeviceID         string `json:"device_id" yaml:"device_id"`
	PCIEBDF          string `json:"pcie_bdf" yaml:"pcie_bdf"`
	PCIESpeed        string `json:"pcie_speed" yaml:"pcie_speed"`
	PCIESpeedState   string `json:"pcie_speed_state" yaml:"pcie_speed_state"`
	PCIEWidth        string `json:"pcie_width" yaml:"pcie_width"`
	PCIEWidthState   string `json:"pcie_width_state" yaml:"pcie_width_state"`
	PCIETreeSpeedMin string `json:"pcie_tree_speed" yaml:"pcie_tree_speed"`
	PCIETreeWidthMin string `json:"pcie_tree_width" yaml:"pcie_tree_width"`
	PCIEMRR          string `json:"pcie_mrr" yaml:"pcie_mrr"`
	// Slot             string `json:"slot" yaml:"slot"`
	NumaNode string `json:"numa_node" yaml:"numa_node"`
	CPULists string `json:"cpu_lists" yaml:"cpu_lists"`
	FWVer    string `json:"fw_ver" yaml:"fw_ver"`
	VPD      string `json:"vpd" yaml:"vpd"`
	OFEDVer  string `json:"ofed_ver" yaml:"ofed_ver"` // compatible with IB Spec Requirement
}

// Collect collects all hardware information for a given IB device and fills the struct.
// port selects which entry under /sys/class/infiniband/<dev>/ports/ is sampled
// (multi-plane HCAs expose more than one). Pass 1 for legacy single-port cards.
func (hw *IBHardWareInfo) Collect(ctx context.Context, IBDev string, port int, ibNicRole string) {
	hw.IBDev = IBDev
	hw.Port = port

	// Basic device information
	hw.HCAType = hw.GetHCAType(IBDev)
	hw.BoardID = hw.GetBoardID(IBDev)
	hw.DeviceID = hw.GetDeviceID(IBDev)
	hw.FWVer = hw.GetFWVer(IBDev)
	hw.VPD = hw.GetVPD(IBDev)

	// GUID information
	hw.SystemGUID = hw.GetSystemGUID(IBDev)
	hw.NodeGUID = hw.GetNodeGUID(IBDev)

	// Port state information
	hw.PhyState = hw.GetPhyStat(IBDev, port)
	hw.PortState = hw.GetIBStat(IBDev, port)
	hw.LinkLayer = hw.GetLinkLayer(IBDev, port)
	hw.PortSpeed = hw.GetPortSpeed(IBDev, port)

	// Network device information.  For multi-plane PFs the per-port netdev
	// lives under ports/<port>/gid_attrs/ndevs/0; fall back to the legacy
	// device-level lookup for single-port cards.
	hw.NetDev = hw.GetPortNetDev(IBDev, port)
	if hw.NetDev == "" {
		hw.NetDev, _ = GetIBdev2NetDev(IBDev)
	}
	hw.NetOperstate = hw.GetNetOperstate(IBDev, hw.NetDev)

	// Gateway information
	hw.PFGW = GetIBGateway().GetPFGW(IBDev)

	// VF information (only for sriovNode)
	if ibNicRole == "sriovNode" {
		hw.VFNum = hw.GetVFNum(IBDev)
		hw.VFSpec = hw.GetVFSpec(IBDev)
	}

	// PCIe information
	if len(GetIBDevBDF(IBDev)) >= 1 {
		hw.PCIEBDF = GetIBDevBDF(IBDev)[0]
	}
	hw.PCIESpeed = GetPCIECLinkSpeed(IBDev)
	hw.PCIEWidth = GetPCIECLinkWidth(IBDev)
	if len(GetPCIEMRR(ctx, IBDev)) >= 1 {
		hw.PCIEMRR = GetPCIEMRR(ctx, IBDev)[0]
	}
	hw.PCIETreeSpeedMin = GetPCIETreeMin(IBDev, "current_link_speed")
	hw.PCIETreeWidthMin = GetPCIETreeMin(IBDev, "current_link_width")
	if len(GetNumaNode(IBDev)) >= 1 {
		hw.NumaNode = GetNumaNode(IBDev)[0]
	}
	if len(GetCPUList(IBDev)) >= 1 {
		hw.CPULists = GetCPUList(IBDev)[0]
	}
}

// GetHCAType gets HCA type
func (c *IBHardWareInfo) GetHCAType(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "hca_type")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read HCA type for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// GetVPD gets VPD information
func (c *IBHardWareInfo) GetVPD(IBDev string) string {
	vpdPath := filepath.Join(IBSYSPathPre, IBDev, "device", "vpd")
	data, err := os.ReadFile(vpdPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to read vpd file %s, err: %v", vpdPath, err)
		return ""
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

// GetFWVer gets firmware version
func (c *IBHardWareInfo) GetFWVer(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "fw_ver")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read firmware version for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// GetBoardID gets board ID
func (c *IBHardWareInfo) GetBoardID(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "board_id")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read board ID for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// readPortAttr reads /sys/class/infiniband/<IBDev>/ports/<port>/<attr> as a single line.
func readPortAttr(IBDev string, port int, attr string) (string, error) {
	rel := fmt.Sprintf("ports/%d/%s", port, attr)
	result, err := ReadIBDevSysfileLines(IBDev, rel)
	if err != nil {
		return "", err
	}
	if len(result) == 0 {
		return "", nil
	}
	return result[0], nil
}

// GetPhyStat gets physical state
func (c *IBHardWareInfo) GetPhyStat(IBDev string, port int) string {
	v, err := readPortAttr(IBDev, port, "phys_state")
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to read physical state for %s/p%d: %v", IBDev, port, err)
	}
	return v
}

// GetIBStat gets IB state
func (c *IBHardWareInfo) GetIBStat(IBDev string, port int) string {
	v, err := readPortAttr(IBDev, port, "state")
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to read IB state for %s/p%d: %v", IBDev, port, err)
	}
	return v
}

// GetPortSpeed gets port speed
func (c *IBHardWareInfo) GetPortSpeed(IBDev string, port int) string {
	v, err := readPortAttr(IBDev, port, "rate")
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to read port speed for %s/p%d: %v", IBDev, port, err)
	}
	return v
}

// GetLinkLayer gets link layer
func (c *IBHardWareInfo) GetLinkLayer(IBDev string, port int) string {
	v, err := readPortAttr(IBDev, port, "link_layer")
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to read link layer for %s/p%d: %v", IBDev, port, err)
	}
	return v
}

// GetDeviceID gets device ID
func (c *IBHardWareInfo) GetDeviceID(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "device/device")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read device ID for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// GetSystemGUID gets system GUID
func (c *IBHardWareInfo) GetSystemGUID(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "sys_image_guid")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read system GUID for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// GetNodeGUID gets node GUID
func (c *IBHardWareInfo) GetNodeGUID(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "node_guid")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read node GUID for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// rdmaLinkNetdevs caches the (IBDev, port) -> netdev mapping derived from
// `rdma link` so multi-plane HCAs do not need to shell out 32 times per
// collection cycle.  The cache is populated lazily on first use and reset
// at the start of every Collect() round.
var (
	rdmaLinkNetdevs   map[string]string
	rdmaLinkNetdevMu  sync.Mutex
	rdmaLinkPopulated bool
)

func rdmaLinkKey(IBDev string, port int) string {
	return fmt.Sprintf("%s/%d", IBDev, port)
}

// resetRDMALinkCache invalidates the cached mapping so the next call
// repopulates from `rdma link`.  Called at the top of InfinibandInfo.Collect.
func resetRDMALinkCache() {
	rdmaLinkNetdevMu.Lock()
	defer rdmaLinkNetdevMu.Unlock()
	rdmaLinkNetdevs = nil
	rdmaLinkPopulated = false
}

func populateRDMALinkCache() {
	rdmaLinkNetdevMu.Lock()
	defer rdmaLinkNetdevMu.Unlock()
	if rdmaLinkPopulated {
		return
	}
	rdmaLinkPopulated = true
	rdmaLinkNetdevs = make(map[string]string)
	out, err := exec.Command("rdma", "link").CombinedOutput()
	if err != nil {
		logrus.WithField("component", "infiniband").Debugf("rdma link unavailable: %v", err)
		return
	}
	// Lines look like:
	//   link roce_r0/3 state ACTIVE physical_state LINK_UP netdev eth_r0_p0
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "link" {
			continue
		}
		dp := strings.SplitN(fields[1], "/", 2)
		if len(dp) != 2 {
			continue
		}
		port, err := strconv.Atoi(dp[1])
		if err != nil {
			continue
		}
		netDev := ""
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "netdev" {
				netDev = fields[i+1]
				break
			}
		}
		if netDev == "" {
			continue
		}
		rdmaLinkNetdevs[rdmaLinkKey(dp[0], port)] = netDev
	}
}

// GetPortNetDev returns the netdev backing a specific port of an HCA.
// Authoritative source is `rdma link`; falls back to the legacy device-level
// lookup (first netdev under /sys/class/infiniband/<dev>/device/net) when
// rdma is unavailable or the port is not enumerated.
func (c *IBHardWareInfo) GetPortNetDev(IBDev string, port int) string {
	populateRDMALinkCache()
	rdmaLinkNetdevMu.Lock()
	netDev, ok := rdmaLinkNetdevs[rdmaLinkKey(IBDev, port)]
	rdmaLinkNetdevMu.Unlock()
	if ok {
		return netDev
	}
	return ""
}

// GetNetOperstate gets network operational state for a specific netdev.
// netDev is the per-port name resolved earlier (e.g. eth_r0_p0); falls back
// to the first netdev under the IB device when empty.
func (c *IBHardWareInfo) GetNetOperstate(IBDev string, netDev string) string {
	if netDev != "" {
		operstatePath := filepath.Join("/sys/class/net", netDev, "operstate")
		if data, err := os.ReadFile(operstatePath); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
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

// GetVFSpec gets VF specification
func (c *IBHardWareInfo) GetVFSpec(IBDev string) string {
	netPath := filepath.Join(IBSYSPathPre, IBDev, "device", "sriov_totalvfs")
	data, err := os.ReadFile(netPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to read net directory %s: %v", netPath, err)
		return ""
	}

	return strings.TrimSpace(string(data))
}

func (i *IBHardWareInfo) GetVFNum(IBDev string) string {
	var vfNum string
	bdf := GetIBDevBDF(IBDev)[0]

	// skip secondary port
	if strings.HasSuffix(bdf, ".1") {
		return "0"
	}

	netDev, _ := GetIBdev2NetDev(IBDev)
	cmd := exec.Command("ip", "link", "show", "dev", netDev)
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

// GetNumaNode gets NUMA node for an IB device
func GetNumaNode(IBDev string) []string {
	BDF := GetIBDevBDF(IBDev)
	if len(BDF) == 0 {
		return nil
	}
	DesPath := path.Join(PCIPath, BDF[0], "numa_node")
	numaNode, err := GetFileCnt(DesPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to read NUMA node for %s: %v", IBDev, err)
		return nil
	}
	return numaNode
}

// GetCPUList gets CPU list for an IB device
func GetCPUList(IBDev string) []string {
	BDF := GetIBDevBDF(IBDev)
	if len(BDF) == 0 {
		return nil
	}
	DesPath := path.Join(PCIPath, BDF[0], "local_cpulist")
	CPUList, err := GetFileCnt(DesPath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to read CPU list for %s: %v", IBDev, err)
		return nil
	}
	return CPUList
}
