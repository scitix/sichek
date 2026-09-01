package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

type InterfaceEntry struct {
	Name     string
	IsIB     bool
	IBDev    string
	IsMLX5   bool   // mlx5 driver (use mlxlink even for ethernet)
	PcieBDF  string // PCIe BDF address for mlxlink -d
}

// isMezzanine reports whether a name belongs to a mezzanine card. The naming
// differs per platform — netdev "mezz0" and IB device "mezz_0" on one, "mezz_0"
// for both on another — so this matches the substring, consistent with how the
// infiniband, hca and perftest packages already exclude these devices.
//
// Mezzanine ports are board-level InfiniBand links with no pluggable optics, so
// there is no transceiver to read: their netdev is IPoIB, where `ethtool -m`
// returns "Operation not supported", and `mlxlink -m` either reports every module
// field as N/A, fails on a firmware ICMD error, or segfaults outright depending
// on the board. Collecting them yields nothing but empty module rows — which look
// like a module with no vendor and 0 °C to the checkers — while costing one slow
// mlxlink invocation per port.
func isMezzanine(name string) bool {
	return strings.Contains(name, "mezz")
}

func EnumerateTransceiverInterfaces() ([]InterfaceEntry, error) {
	var entries []InterfaceEntry

	netDir := "/sys/class/net"
	netEntries, err := os.ReadDir(netDir)
	if err != nil {
		return nil, err
	}
	for _, e := range netEntries {
		name := e.Name()
		// Skip loopback, virtual, and container interfaces
		if name == "lo" || name == "bonding_masters" ||
			strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "virbr") {
			continue
		}
		// Skip mezzanine ports — no pluggable transceiver to read.
		if isMezzanine(name) {
			continue
		}
		// Skip VLAN sub-interfaces (e.g. eth0.10, eth1.2)
		if strings.Contains(name, ".") {
			continue
		}
		// Must have a physical device backing it
		devicePath := filepath.Join(netDir, name, "device")
		if _, err := os.Stat(devicePath); os.IsNotExist(err) {
			continue
		}
		// Skip SR-IOV Virtual Functions — physfn exists only on VFs, not PFs.
		// mlxlink on VF BDFs fails slowly (8-11s), wasting the collection budget.
		physfnPath := filepath.Join(netDir, name, "device", "physfn")
		if _, err := os.Stat(physfnPath); err == nil {
			continue
		}
		// Detect mlx5 driver — these need mlxlink instead of ethtool for DOM
		entry := InterfaceEntry{Name: name, IsIB: false}
		driverLink := filepath.Join(netDir, name, "device", "driver")
		if target, err := os.Readlink(driverLink); err == nil {
			driverName := filepath.Base(target)
			if driverName == "mlx5_core" {
				entry.IsMLX5 = true
				// Read PCIe BDF from uevent
				ueventPath := filepath.Join(netDir, name, "device", "uevent")
				if data, err := os.ReadFile(ueventPath); err == nil {
					for _, line := range strings.Split(string(data), "\n") {
						if strings.HasPrefix(line, "PCI_SLOT_NAME=") {
							entry.PcieBDF = strings.TrimPrefix(line, "PCI_SLOT_NAME=")
							break
						}
					}
				}
			}
		}
		entries = append(entries, entry)
	}

	ibDir := "/sys/class/infiniband"
	ibEntries, err := os.ReadDir(ibDir)
	if err != nil {
		logrus.WithField("component", "transceiver").Debugf("no infiniband devices: %v", err)
		return entries, nil
	}
	for _, e := range ibEntries {
		ibDev := e.Name()
		// Skip mezzanine ports here too: they are enumerated as IB devices
		// ("mezz_0"), which is the path that would otherwise reach mlxlink.
		if isMezzanine(ibDev) {
			continue
		}
		physfn := filepath.Join(ibDir, ibDev, "device", "physfn")
		if _, err := os.Stat(physfn); err == nil {
			continue
		}
		netDevDir := filepath.Join(ibDir, ibDev, "device", "net")
		netDevs, _ := os.ReadDir(netDevDir)
		netDevName := ""
		var netBDF string
		if len(netDevs) > 0 {
			netDevName = netDevs[0].Name()
			ueventPath := filepath.Join(netDir, netDevName, "device", "uevent")
			if data, err := os.ReadFile(ueventPath); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.HasPrefix(line, "PCI_SLOT_NAME=") {
						netBDF = strings.TrimPrefix(line, "PCI_SLOT_NAME=")
						break
					}
				}
			}
		}
		entries = append(entries, InterfaceEntry{Name: netDevName, IsIB: true, IBDev: ibDev, PcieBDF: netBDF})
	}

	return entries, nil
}

type NetworkClassifier struct {
	patterns         map[string][]string // network_type -> interface patterns
	managementMaxMbps int                 // speed <= this is management (0 = disabled)
}

func NewNetworkClassifier(patterns map[string][]string, managementMaxMbps int) *NetworkClassifier {
	return &NetworkClassifier{patterns: patterns, managementMaxMbps: managementMaxMbps}
}

// Classify determines network type. Speed-based classification takes priority over pattern matching.
func (c *NetworkClassifier) Classify(ifaceName string, speedMbps int) string {
	// Speed-based classification first (most reliable)
	if c.managementMaxMbps > 0 && speedMbps > 0 && speedMbps <= c.managementMaxMbps {
		return "management"
	}

	// Fallback to pattern matching
	if patterns, ok := c.patterns["management"]; ok {
		for _, p := range patterns {
			if matched, _ := filepath.Match(p, ifaceName); matched {
				return "management"
			}
		}
	}
	if patterns, ok := c.patterns["business"]; ok {
		for _, p := range patterns {
			if matched, _ := filepath.Match(p, ifaceName); matched {
				return "business"
			}
		}
	}
	return "business"
}

// GetLinkSpeed reads the interface link speed from sysfs and returns (human-readable string, numeric Mbps).
func GetLinkSpeed(ifaceName string) (string, int) {
	if ifaceName == "" {
		return "", 0
	}
	speedPath := filepath.Join("/sys/class/net", ifaceName, "speed")
	data, err := os.ReadFile(speedPath)
	if err != nil {
		return "", 0
	}
	speedStr := strings.TrimSpace(string(data))
	if speedStr == "" || speedStr == "-1" {
		return "", 0
	}
	speedInt := 0
	fmt.Sscanf(speedStr, "%d", &speedInt)
	return speedStr + " Mbps", speedInt
}
