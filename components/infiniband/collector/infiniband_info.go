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
	"sort"
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
	IBVendorIDs     = []string{
		"0x15b3", // Mellanox Technologies
	}

	IBDeviceIDs = []string{
		"0x1021", // CMT2910 Family [ConnectX-7]
		"0x1023", // CX8 Family [ConnectX-8]
		"0xa2dc", // BlueField-3 E-series SuperNIC
		"0x09a2", // CMT2910 Family [ConnectX-7] HHHL
		"0x2330", // HPE/Enhance 400G
		"0x4128",
		"0x02b2",
	}
)

type InfinibandInfo struct {
	HCAPCINum      int                           `json:"hca_pci_num" yaml:"hca_pci_num"`
	IBPFDevs       map[string]string             `json:"ib_dev" yaml:"ib_dev"`
	IBPCIDevs      map[string]string             `json:"hca_pci_dev" yaml:"hca_pci_dev"`
	IBHardWareInfo map[string]IBHardWareInfo     `json:"ib_hardware_info" yaml:"ib_hardware_info"`
	IBSoftWareInfo IBSoftWareInfo                `json:"ib_software_info" yaml:"ib_software_info"`
	PCIETreeInfo   map[string]PCIETreeInfo       `json:"pcie_tree_info" yaml:"pcie_tree_info"`
	IBCounters     map[string]map[string]uint64  `json:"ib_counters" yaml:"ib_counters"`
	IBNicRole      string                        `json:"ib_nic_role" yaml:"ib_nic_role"`
	Time           time.Time                     `json:"time" yaml:"time"`
	GWCache        map[string]*gatewayCacheEntry `json:"gateway_cache" yaml:"gateway_cache"`
	mu             sync.RWMutex
}

type gatewayCacheEntry struct {
	GatewayIP string    `json:"gateway_ip" yaml:"gateway_ip"`
	Err       error     `json:"error" yaml:"error"`
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
}

type IBHardWareInfo struct {
	IBDev            string `json:"IBdev" yaml:"IBdev"`
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
	Slot             string `json:"slot" yaml:"slot"`
	NumaNode         string `json:"numa_node" yaml:"numa_node"`
	CPULists         string `json:"cpu_lists" yaml:"cpu_lists"`
	FWVer            string `json:"fw_ver" yaml:"fw_ver"`
	VPD              string `json:"vpd" yaml:"vpd"`
	OFEDVer          string `json:"ofed_ver" yaml:"ofed_ver"` // compatible with IB Spec Requirement
}

type PCIETreeInfo struct {
	PCIETreeSpeed []PCIETreeSpeedInfo `json:"pcie_tree_speed" yaml:"pcie_tree_speed"`
	PCIETreeWidth []PCIETreeWidthInfo `json:"pcie_tree_width" yaml:"pcie_tree_width"`
}

type PCIETreeSpeedInfo struct {
	BDF   string `json:"bdf" yaml:"bdf"`
	Speed string `json:"speed" yaml:"speed"`
}

type PCIETreeWidthInfo struct {
	BDF   string `json:"bdf" yaml:"bdf"`
	Width string `json:"width" yaml:"width"`
}

type IBSoftWareInfo struct {
	OFEDVer      string   `json:"ofed_ver" yaml:"ofed_ver"`
	KernelModule []string `json:"kernel_module" yaml:"kernel_module"`
}

func (i *InfinibandInfo) JSON() (string, error) {
	data, err := json.Marshal(i)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RLock acquires a read lock for safe concurrent read access
func (i *InfinibandInfo) RLock() {
	i.mu.RLock()
}

// RUnlock releases a read lock
func (i *InfinibandInfo) RUnlock() {
	i.mu.RUnlock()
}

// Lock acquires a write lock for safe concurrent write access
func (i *InfinibandInfo) Lock() {
	i.mu.Lock()
}

// Unlock releases a write lock
func (i *InfinibandInfo) Unlock() {
	i.mu.Unlock()
}

func (i *InfinibandInfo) GetPFDevs(IBDevs []string) []string {
	PFDevs := make([]string, 0)
	for _, IBDev := range IBDevs {

		// ignore cx4 interface
		hcaTypePath := path.Join(IBSYSPathPre, IBDev, "hca_type")
		if _, err := os.Stat(hcaTypePath); err != nil {
			continue
		}
		hcaTypeContent, err := os.ReadFile(hcaTypePath)
		if err == nil {
			hcaType := strings.TrimSpace(string(hcaTypeContent))
			if strings.Contains(hcaType, "MT4117") {
				continue
			}
		}

		// ignore virtual functions
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
	re, _ := regexp.Compile(`^bond[0-9]$`)
	for _, IBDev := range PFDevs {
		if re.MatchString(IBDev) {
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
	logrus.WithField("component", "infiniband").Debugf("get the IB and net map: %v", IBPFDevs)

	return IBPFDevs
}

func (i *InfinibandInfo) FindIBPCIDevices(targetVendorID []string, targetDeviceIDs []string) (map[string]string, error) {
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

	log.Debugf("Scanning PCI devices in %s for vendor ID %s and device IDs %v", PCIPath, targetVendorID, targetDeviceIDs)

	foundDevices := make(map[string]string)

	for _, entry := range entries {
		pciAddr := entry.Name()
		deviceDir := filepath.Join(PCIPath, pciAddr)

		// Read and compare vendor ID
		vendorBytes, err := os.ReadFile(filepath.Join(deviceDir, "vendor"))
		if err != nil {
			log.Warnf("Could not read vendor file for %s, skipping. Error: %v", pciAddr, err)
			continue
		}
		currentVendorID := strings.TrimSpace(string(vendorBytes))

		// If vendor ID doesn't match, skip this device directly
		if !slices.Contains(targetVendorID, currentVendorID) {
			continue
		}

		// Vendor ID matches, then read device ID
		deviceBytes, err := os.ReadFile(filepath.Join(deviceDir, "device"))
		if err != nil {
			log.Warnf("Could not read device file for %s, skipping. Error: %v", pciAddr, err)
			continue
		}
		currentDeviceID := strings.TrimSpace(string(deviceBytes))

		// Check if device ID is in the target list
		if slices.Contains(targetDeviceIDs, currentDeviceID) {
			log.Debugf("Found matching device: %s with vendor=%s, device=%s ", pciAddr, currentVendorID, currentDeviceID)
			foundDevices[pciAddr] = fmt.Sprintf("%s:%s", currentVendorID, currentDeviceID)
		}
	}

	log.Debugf("Finished PCI scan. Found %d matching devices.", len(foundDevices))
	return foundDevices, nil
}

func (i *InfinibandInfo) GetBondInterface(slaveInterface string) (string, error) {
	bondPattern := "/sys/class/net/bond*"

	bondDirs, err := filepath.Glob(bondPattern)
	if err != nil {
		return "", err
	}

	for _, bondDir := range bondDirs {
		slavesFile := filepath.Join(bondDir, "bonding/slaves")
		data, err := os.ReadFile(slavesFile)
		if err != nil {
			continue
		}

		slaves := strings.Fields(string(data))
		for _, slave := range slaves {
			if slave == slaveInterface {
				return filepath.Base(bondDir), nil
			}
		}
	}

	return "", fmt.Errorf("no bond interface found for %s", slaveInterface)
}

// GetNetInterfaceFromIB returns the final network interface (bond or physical)
func (i *InfinibandInfo) GetNetInterfaceFromIB(ibDevice string) (string, error) {
	// Step 1: Get the physical interface
	netPath := filepath.Join("/sys/class/infiniband", ibDevice, "device/net")

	entries, err := os.ReadDir(netPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", netPath, err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no network interface found for %s", ibDevice)
	}

	physicalIface := entries[0].Name()

	// Step 2: Check if this interface is part of a bond
	bondIface, err := i.GetBondInterface(physicalIface)
	if err == nil {
		// It's part of a bond, return the bond interface
		return bondIface, nil
	}

	// Step 3: Not part of a bond, return the physical interface
	return physicalIface, nil
}

func (i *InfinibandInfo) GetIBdev2NetDev(IBDev string) []string {
	if bonded, _ := i.IsNicBonded(); bonded {
		// If the NIC is bonded, we need to find the active slave
		bond, _ := i.GetNetInterfaceFromIB(IBDev)
		// GetNetInterfaceFromIB returns a single interface name (string) for bonded devices;
		// wrap it into a slice to match the function's []string return type.
		if bond == "" {
			return []string{}
		}
		return []string{bond}
	}
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

// Prioritize querying policy routing, if not found then query interface routing
func (i *InfinibandInfo) _findGatewayWithNetlink(ifaceName string) (string, error) {
	// Get interface Link object by interface name
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("netlink: failed to find interface '%s': %w", ifaceName, err)
	}

	// --- 1. Prioritize querying policy routing ---
	gateway, err := i._findGatewayFromPolicyRouting(link)
	if err == nil && gateway != "" {
		logrus.WithFields(logrus.Fields{
			"component": "infiniband",
		}).Infof("Found gateway via policy routing, interface:%s gateway:%s ", ifaceName, gateway)
		return gateway, nil
	}

	logrus.WithFields(logrus.Fields{
		"component": "infiniband",
		"interface": ifaceName,
		"error":     err,
	}).Debug("Policy routing lookup failed, falling back to interface routing")

	// --- 2. Fall back to interface routing query ---
	return i._findGatewayFromInterfaceRouting(link, ifaceName)
}

// _findGatewayFromPolicyRouting finds gateway through policy routing
func (i *InfinibandInfo) _findGatewayFromPolicyRouting(link netlink.Link) (string, error) {
	// Get interface IPv4 addresses as source addresses
	sourceIPs, err := i._getInterfaceIPv4Addresses(link)
	if err != nil {
		return "", fmt.Errorf("failed to get interface addresses: %w", err)
	}

	if len(sourceIPs) == 0 {
		return "", fmt.Errorf("no IPv4 address found on interface")
	}

	// Get routing rules
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		return "", fmt.Errorf("failed to list routing rules: %w", err)
	}

	// Sort rules by priority (smaller priority values processed first)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	// Iterate through each source IP address
	for _, sourceIP := range sourceIPs {
		logrus.WithFields(logrus.Fields{
			"component": "infiniband",
			"source_ip": sourceIP.String(),
		}).Debug("Checking policy rules for source IP")

		// Check matching routing rules - modified here, pass struct instead of pointer
		for _, rule := range rules {
			if i._isRuleMatchingSource(rule, sourceIP) {
				gateway, err := i._findGatewayInTable(rule.Table, sourceIP)
				if err == nil && gateway != "" {
					logrus.WithField("component", "infiniband").Infof("Found gateway in policy table - source_ip: %s, table: %d, priority: %d, gateway: %s",
						sourceIP.String(), rule.Table, rule.Priority, gateway)

					return gateway, nil
				}
				logrus.WithFields(logrus.Fields{
					"component": "infiniband",
					"table":     rule.Table,
					"error":     err,
				}).Debug("No gateway found in policy table")
			}
		}
	}

	return "", fmt.Errorf("no gateway found in policy routing tables")
}

// _isRuleMatchingSource checks if routing rule matches source address - modified parameter type
func (i *InfinibandInfo) _isRuleMatchingSource(rule netlink.Rule, sourceIP net.IP) bool {
	// Check if rule specifies source address matching
	if rule.Src != nil {
		// Exact match source IP
		if rule.Src.IP.Equal(sourceIP) {
			return true
		}
		// Network match (if rule specifies network segment)
		if rule.Src.Contains(sourceIP) {
			return true
		}
		return false
	}

	// If rule doesn't specify source address (from all), match all
	return true
}

// _findGatewayInTable finds gateway in specified routing table
func (i *InfinibandInfo) _findGatewayInTable(tableID int, sourceIP net.IP) (string, error) {
	// Get routes in specified table
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{
		Table: tableID,
	}, netlink.RT_FILTER_TABLE)

	if err != nil {
		return "", fmt.Errorf("failed to list routes for table %d: %w", tableID, err)
	}

	logrus.WithFields(logrus.Fields{
		"component": "infiniband",
		"table":     tableID,
		"routes":    len(routes),
	}).Debug("Checking routes in policy table")

	var fallbackGateway net.IP

	// Find default route
	for _, route := range routes {
		if route.Dst == nil && route.Gw != nil {
			logrus.WithFields(logrus.Fields{
				"component":  "infiniband",
				"table":      tableID,
				"gateway":    route.Gw.String(),
				"route_type": "default",
			}).Debug("Found default route in policy table")
			return route.Gw.String(), nil
		}

		// Record fallback gateway
		if fallbackGateway == nil && route.Gw != nil {
			fallbackGateway = route.Gw
		}
	}

	// Return fallback gateway
	if fallbackGateway != nil {
		logrus.WithFields(logrus.Fields{
			"component":  "infiniband",
			"table":      tableID,
			"gateway":    fallbackGateway.String(),
			"route_type": "fallback",
		}).Debug("Using fallback gateway from policy table")
		return fallbackGateway.String(), nil
	}

	return "", fmt.Errorf("no gateway found in table %d", tableID)
}

// _getInterfaceIPv4Addresses gets interface IPv4 addresses
func (i *InfinibandInfo) _getInterfaceIPv4Addresses(link netlink.Link) ([]net.IP, error) {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}

	var ips []net.IP
	for _, addr := range addrs {
		if addr.IP.To4() != nil { // Ensure it's an IPv4 address
			ips = append(ips, addr.IP)
		}
	}

	return ips, nil
}

// _findGatewayFromInterfaceRouting finds gateway through interface routing (original logic)
func (i *InfinibandInfo) _findGatewayFromInterfaceRouting(link netlink.Link, ifaceName string) (string, error) {
	// Only find IPv4 routes associated with this interface
	routes, err := netlink.RouteList(link, netlink.FAMILY_V4)
	if err != nil {
		return "", fmt.Errorf("netlink: failed to list routes for '%s': %w", ifaceName, err)
	}

	logrus.WithFields(logrus.Fields{
		"component": "infiniband",
		"interface": ifaceName,
		"routes":    len(routes),
	}).Debug("Checking interface routes")

	// Iterate through routing table, looking for gateway
	var fallbackGateway net.IP
	for _, route := range routes {
		// Prioritize returning default route
		if route.Dst == nil && route.Gw != nil {
			logrus.WithFields(logrus.Fields{
				"component": "infiniband",
				"interface": ifaceName,
				"gateway":   route.Gw.String(),
				"method":    "interface_routing",
			}).Info("Found default gateway via interface routing")
			return route.Gw.String(), nil
		}

		// Record fallback gateway
		if fallbackGateway == nil && route.Gw != nil {
			fallbackGateway = route.Gw
		}
	}

	// Return fallback gateway
	if fallbackGateway != nil {
		logrus.WithFields(logrus.Fields{
			"component": "infiniband",
			"interface": ifaceName,
			"gateway":   fallbackGateway.String(),
			"method":    "interface_routing_fallback",
		}).Info("Using fallback gateway via interface routing")
		return fallbackGateway.String(), nil
	}

	// If nothing found
	return "", fmt.Errorf("netlink: no gateway found for interface '%s'", ifaceName)
}

func (i *InfinibandInfo) FindGateway(ifaceName string) (string, error) {
	// --- 1. Fast path: Use read lock to check cache ---
	i.mu.RLock()
	entry, exists := i.GWCache[ifaceName]
	if exists && time.Since(entry.Timestamp) < gatewayCacheTTL {
		logrus.WithField("component", "infiniband").Infof("Gateway cache hit for interface %s: %s", ifaceName, entry.GatewayIP)
		i.mu.RUnlock()
		return entry.GatewayIP, entry.Err // Directly return cached result
	}
	i.mu.RUnlock()

	// --- 2. Slow path: Use write lock to execute query and update cache ---
	i.mu.Lock()
	defer i.mu.Unlock()

	// --- Double-check locking: While waiting for write lock, another goroutine may have completed refresh ---
	entry, exists = i.GWCache[ifaceName]
	if exists && time.Since(entry.Timestamp) < gatewayCacheTTL {
		logrus.WithField("component", "infiniband").Infof("Gateway cache hit after lock for interface %s.", ifaceName)
		return entry.GatewayIP, entry.Err
	}

	// --- Cache miss or expired, execute actual query ---
	gateway, err := i._findGatewayWithNetlink(ifaceName)

	// --- Write new result to cache ---
	i.GWCache[ifaceName] = &gatewayCacheEntry{
		GatewayIP: gateway,
		Err:       err,
		Timestamp: time.Now(),
	}

	return gateway, err
}

func (i *InfinibandInfo) CheckIPVersionViaSysfs(interfaceName string) (hasIPv4, hasIPv6 bool, err error) {
	basePath := filepath.Join("/sys/class/net", interfaceName)

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return false, false, fmt.Errorf("interface %s not found", interfaceName)
	}

	ipv4Path := "/proc/net/fib_trie"
	if data, err := os.ReadFile(ipv4Path); err == nil {
		if strings.Contains(string(data), interfaceName) {
			hasIPv4 = true
		}
	}

	ipv6Path := "/proc/net/if_inet6"
	if data, err := os.ReadFile(ipv6Path); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, interfaceName) {
				hasIPv6 = true
				break
			}
		}
	}

	return hasIPv4, hasIPv6, nil
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
		_, hasIPv6, err := i.CheckIPVersionViaSysfs(ifaceName)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to check IP version for interface %s: %v", ifaceName, err)
			return ""
		}
		if hasIPv6 {
			logrus.WithField("component", "infiniband").Infof("Interface %s has IPv6 address, skipping gateway lookup for IB device: %s", ifaceName, IBDev)
			return "IPV6"
		}
		logrus.WithField("component", "infiniband").Infof("Interface %s has IPv4 address, continuing gateway lookup for IB device: %s", ifaceName, IBDev)

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
	var mu sync.Mutex
	counterTypes := []string{"counters", "hw_counters"}

	wg.Add(len(counterTypes))
	for _, counterType := range counterTypes {
		go func(ct string) {
			defer wg.Done()
			counter, err := i.GetIBCounter(IBDev, ct)
			if err != nil {
				logrus.WithField("component", "infiniband").Errorf("Get IB Counter failed, err:%s", err)
				return
			}
			// Use mutex to protect concurrent writes to map
			mu.Lock()
			for k, v := range counter {
				Counters[k] = v
			}
			mu.Unlock()
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
		// autofix
		if strings.Compare(mrr[0], "4096") != 0 {
			// get BDF
			bdf := i.GetBDF(IBDev)
			// autofix
			i.ModifyPCIeMaxReadRequest(bdf[0], "68", 5)
		}
	}

	return mrr
}

// ModifyPCIeMaxReadRequest modifies the Max Read Request Size of a PCIe device
// deviceAddr: PCI device address, e.g., "80:00.0"
// offset: Register offset address, e.g., "68"
// newHighNibble: New high nibble value (0-F)
func (i *InfinibandInfo) ModifyPCIeMaxReadRequest(deviceAddr string, offset string, newHighNibble int) error {
	// Validate input parameters
	if newHighNibble < 0 || newHighNibble > 0xF {
		return fmt.Errorf("new high nibble value must be between 0-F")
	}

	// Read current value
	readCmd := exec.Command("setpci", "-s", deviceAddr, offset+".w")
	output, err := readCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read PCI register: %v", err)
	}

	// Parse the returned hexadecimal value
	currentValueStr := strings.TrimSpace(string(output))
	currentValue, err := strconv.ParseUint(currentValueStr, 16, 16)
	if err != nil {
		return fmt.Errorf("failed to parse hex value: %v", err)
	}

	// fmt.Printf("Current value: 0x%04X\n", currentValue)

	// Modify the high nibble
	// Clear the top 4 bits (0x0FFF mask)
	newValue := currentValue & 0x0FFF
	// Set the new high nibble
	newValue |= uint64(newHighNibble) << 12

	logrus.WithField("component", "infiniband").Infof("Modifying PCIe Max Read Request for device %s at offset %s: current value 0x%04X, new value 0x%04X", deviceAddr, offset, currentValue, newValue)
	// fmt.Printf("New value: 0x%04X\n", newValue)

	// Write back the new value
	writeValueStr := fmt.Sprintf("%04x", newValue)
	writeCmd := exec.Command("setpci", "-s", deviceAddr, offset+".w="+writeValueStr)
	err = writeCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to write PCI register: %v", err)
	}

	// Verify the write was successful
	verifyCmd := exec.Command("setpci", "-s", deviceAddr, offset+".w")
	verifyOutput, err := verifyCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to verify write result: %v", err)
	}

	verifiedValueStr := strings.TrimSpace(string(verifyOutput))
	verifiedValue, err := strconv.ParseUint(verifiedValueStr, 16, 16)
	if err != nil {
		return fmt.Errorf("failed to parse verification value: %v", err)
	}

	if verifiedValue != newValue {
		return fmt.Errorf("write verification failed: expected 0x%04X, got 0x%04X", newValue, verifiedValue)
	}

	fmt.Printf("Successfully modified! Verified value: 0x%04X\n", verifiedValue)
	return nil
}

func (i *InfinibandInfo) GetPCIETreeMin(IBDev, linkType string) string {
	bdfList := i.GetBDF(IBDev)
	if len(bdfList) == 0 {
		logrus.WithField("component", "infiniband").Warnf("Could not get BDF for IB device %s", IBDev)
		return ""
	}
	// bdf is the BDF address of the terminal device itself
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

	// Filter out the device's own BDF
	var upstreamBdfs []string
	for _, foundBdf := range allBdfsInPath {
		if foundBdf != bdf {
			upstreamBdfs = append(upstreamBdfs, foundBdf)
		}
	}

	if len(upstreamBdfs) == 0 {
		// If there are no upstream devices (e.g., device directly connected to CPU), this is normal, just return
		logrus.WithField("component", "infiniband").Infof("No upstream PCIe devices found in path for %s, skipping check.", bdf)
		return ""
	}

	if len(upstreamBdfs) == 1 {
		// If there's only one upstream device, it means it's directly connected to CPU, this is also normal, just return
		logrus.WithField("component", "infiniband").Infof("Only one upstream PCIe device found in path for %s, likely direct to CPU, skipping check.", bdf)
		return ""
	}

	logrus.WithField("component", "infiniband").Infof("Checking upstream devices for %s: %v", bdf, upstreamBdfs)

	var minNumericString string
	minNumericValue := math.MaxFloat64

	// Now, we only iterate through the BDF list of upstream devices
	for _, currentBdf := range upstreamBdfs {
		logrus.WithField("component", "infiniband").Infof("Checking upstream device %s for property %s", currentBdf, linkType)
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
			logrus.WithField("component", "infiniband").Debugf(
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
	logrus.WithField("component", "infiniband").Infof("get the pcie tree speed, ib:%s bdfs:%v", IBDev, bdfs)

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

	if _, err := exec.LookPath("ofed_info"); err == nil {
		if output, err := exec.CommandContext(ctx, "ofed_info", "-s").Output(); err == nil {
			if ver := strings.Split(string(output), ":")[0]; ver != "" {
				return ver
			}
		}
	}

	if data, err := os.ReadFile("/sys/module/mlx5_core/version"); err == nil {
		if ver := strings.TrimSpace(string(data)); ver != "" {
			return fmt.Sprintf("rdma_core:%s", ver)
		}
	}
	return "Not Get"
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
	i.IBPCIDevs, _ = i.FindIBPCIDevices(IBVendorIDs, IBDeviceIDs)
	i.IBPFDevs = i.GetIBPFdevs()
	var pciNum int
	for IBDev := range i.IBPFDevs {
		// Handle bond interface
		if strings.Contains(IBDev, "mlx5_bond") {
			// skip mgt bond intrface
			hcaTypePath := path.Join(IBSYSPathPre, IBDev, "hca_type")
			contentBytes, err := os.ReadFile(hcaTypePath)
			if err != nil {

				fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
				os.Exit(1)
			}

			content := strings.TrimSpace(string(contentBytes))
			if strings.Contains(content, "MT4119") {
				continue
			}
			pciNum += 2
		} else {
			pciNum += 1
		}
	}
	i.HCAPCINum = pciNum
	for IBDev := range i.IBPFDevs {
		// skip mezzanine card
		if strings.Contains(IBDev, "mezz") {
			continue
		}

		bdf := i.GetBDF(IBDev)[0]
		if len(bdf) > 0 && strings.HasSuffix(bdf, ".1") {
			netDir := fmt.Sprintf("/sys/bus/pci/devices/%s/net", bdf)
			files, err := os.ReadDir(netDir)
			if err != nil {
				logrus.WithField("component", "infiniband").Errorf("Error reading net dir (driver loaded?): %v", err)
				continue
			}

			if len(files) == 0 {
				logrus.WithField("component", "infiniband").Errorf("No network interface found for this BDF: %s", bdf)
				continue
			}
			masterPath := fmt.Sprintf("/sys/class/net/%s/master", files[0].Name())
			_, err = os.Lstat(masterPath)
			if os.IsNotExist(err) {
				continue
			}
		}

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
func (i *InfinibandInfo) IsNicBonded() (bool, error) {
	re := regexp.MustCompile(`^mlx5_bond_[0-9]$`)
	entries, err := os.ReadDir(IBSYSPathPre)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("could not read directory '%s': %w", IBSYSPathPre, err)
	}

	for _, entry := range entries {
		if re.MatchString(entry.Name()) {
			return true, nil
		}
	}
	logrus.WithField("component", "infiniband").Infof("Info: No devices matching the pattern 'mlx5_bond_[0-9]' were found in '%s'.", IBSYSPathPre)
	return false, nil
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
	// Get PCIe device list
	i.IBPCIDevs, _ = i.FindIBPCIDevices(IBVendorIDs, IBDeviceIDs)
	// Get device list through driver
	i.IBPFDevs = i.GetIBPFdevs()
	// i.HCAPCINum = len(i.IBPFDevs)

	var pciNum int
	for IBDev := range i.IBPFDevs {
		// Handle bond interface
		if strings.Contains(IBDev, "mlx5_bond") {
			// skip mgt bond intrface
			hcaTypePath := path.Join(IBSYSPathPre, IBDev, "hca_type")
			contentBytes, err := os.ReadFile(hcaTypePath)
			if err != nil {

				fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
				os.Exit(1)
			}

			content := strings.TrimSpace(string(contentBytes))
			if strings.Contains(content, "MT4119") {
				continue
			}
			pciNum += 2
		} else {
			pciNum += 1
		}
	}
	i.HCAPCINum = pciNum

	i.IBNicRole = i.GetNICRole()

	i.IBSoftWareInfo.OFEDVer = strings.TrimPrefix(i.GetOFEDInfo(ctx), "rdma-core:")
	i.IBSoftWareInfo.KernelModule = i.GetKernelModule()

	for IBDev := range i.IBPFDevs {
		// skip one card two poor and not bonded
		bdf := i.GetBDF(IBDev)[0]
		if len(bdf) > 0 && strings.HasSuffix(bdf, ".1") {
			netDir := fmt.Sprintf("/sys/bus/pci/devices/%s/net", bdf)
			files, err := os.ReadDir(netDir)
			if err != nil {
				logrus.WithField("component", "infiniband").Errorf("Error reading net dir (driver loaded?): %v", err)
				continue
			}

			if len(files) == 0 {
				logrus.WithField("component", "infiniband").Errorf("No network interface found for this BDF: %s", bdf)
				continue
			}
			masterPath := fmt.Sprintf("/sys/class/net/%s/master", files[0].Name())
			_, err = os.Lstat(masterPath)
			if os.IsNotExist(err) {
				continue
			}
		}
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
		// skip mezzanine card for counters
		// as: Fail to read the ib counter from path: /sys/class/infiniband/mezz_0/ports/1/counters/VL15_dropped
		if strings.Contains(IBDev, "mezz") {
			continue
		}

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
