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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type gatewayCacheEntry struct {
	GatewayIP string    `json:"gateway_ip" yaml:"gateway_ip"`
	Err       error     `json:"error" yaml:"error"`
	Timestamp time.Time `json:"timestamp" yaml:"timestamp"`
}

type IBGateway struct {
	GWCache map[string]*gatewayCacheEntry
	mu      sync.RWMutex
}

var (
	ibGatewayOnce sync.Once
	ibGatewayInst *IBGateway
)

func GetIBGateway() *IBGateway {
	ibGatewayOnce.Do(func() {
		ibGatewayInst = &IBGateway{
			GWCache: make(map[string]*gatewayCacheEntry),
		}
	})
	return ibGatewayInst
}

// GetPFGW gets PF gateway for an IB device
// This method handles gateway lookup with caching and provides a clean interface for other modules
func (gw *IBGateway) GetPFGW(IBDev string) string {
	linkLayer := gw.GetIBDevLinklayer(IBDev)

	switch linkLayer {
	case "InfiniBand":
		logrus.WithField("component", "infiniband").Debugf("No gateway for InfiniBand device: %s", IBDev)
		return ""

	case "Ethernet":
		ifaceName := GetIBdev2NetDev(IBDev)
		_, hasIPv6, err := gw.CheckIPVersionViaSysfs(ifaceName)
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

		// Use cached gateway lookup
		gatewayIP, err := gw.FindGateway(ifaceName)
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

func (gw *IBGateway) GetIBDevLinklayer(IBDev string) string {
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

// FindGateway finds gateway for an interface
func (gw *IBGateway) FindGateway(ifaceName string) (string, error) {
	// --- 1. Fast path: Use read lock to check cache ---
	gw.mu.RLock()
	entry, exists := gw.GWCache[ifaceName]
	if exists && time.Since(entry.Timestamp) < gatewayCacheTTL {
		logrus.WithField("component", "infiniband").Infof("Gateway cache hit for interface %s: %s", ifaceName, entry.GatewayIP)
		gw.mu.RUnlock()
		return entry.GatewayIP, entry.Err // Directly return cached result
	}
	gw.mu.RUnlock()

	// --- 2. Slow path: Use write lock to execute query and update cache ---
	gw.mu.Lock()
	defer gw.mu.Unlock()

	// --- Double-check locking: While waiting for write lock, another goroutine may have completed refresh ---
	entry, exists = gw.GWCache[ifaceName]
	if exists && time.Since(entry.Timestamp) < gatewayCacheTTL {
		logrus.WithField("component", "infiniband").Infof("Gateway cache hit after lock for interface %s.", ifaceName)
		return entry.GatewayIP, entry.Err
	}

	// --- Cache miss or expired, execute actual query ---
	gateway, err := gw._findGatewayWithNetlink(ifaceName)

	// --- Write new result to cache ---
	gw.GWCache[ifaceName] = &gatewayCacheEntry{
		GatewayIP: gateway,
		Err:       err,
		Timestamp: time.Now(),
	}

	return gateway, err
}

// Prioritize querying policy routing, if not found then query interface routing
func (gw *IBGateway) _findGatewayWithNetlink(ifaceName string) (string, error) {
	// Get interface Link object by interface name
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("netlink: failed to find interface '%s': %w", ifaceName, err)
	}

	// --- 1. Prioritize querying policy routing ---
	gateway, err := gw._findGatewayFromPolicyRouting(link)
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
	return gw._findGatewayFromInterfaceRouting(link, ifaceName)
}

// _findGatewayFromPolicyRouting finds gateway through policy routing
func (gw *IBGateway) _findGatewayFromPolicyRouting(link netlink.Link) (string, error) {
	// Get interface IPv4 addresses as source addresses
	sourceIPs, err := gw._getInterfaceIPv4Addresses(link)
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
			if gw._isRuleMatchingSource(rule, sourceIP) {
				gateway, err := gw._findGatewayInTable(rule.Table /*sourceIP*/)
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
func (gw *IBGateway) _isRuleMatchingSource(rule netlink.Rule, sourceIP net.IP) bool {
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
func (gw *IBGateway) _findGatewayInTable(tableID int /*sourceIP net.IP*/) (string, error) {
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
func (gw *IBGateway) _getInterfaceIPv4Addresses(link netlink.Link) ([]net.IP, error) {
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
func (gw *IBGateway) _findGatewayFromInterfaceRouting(link netlink.Link, ifaceName string) (string, error) {
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

func (gw *IBGateway) CheckIPVersionViaSysfs(interfaceName string) (hasIPv4, hasIPv6 bool, err error) {
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
