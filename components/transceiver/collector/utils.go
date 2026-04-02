package collector

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

type InterfaceEntry struct {
	Name  string
	IsIB  bool
	IBDev string
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
		// Skip VLAN sub-interfaces (e.g. eth0.10, eth1.2)
		if strings.Contains(name, ".") {
			continue
		}
		// Must have a physical device backing it
		devicePath := filepath.Join(netDir, name, "device")
		if _, err := os.Stat(devicePath); os.IsNotExist(err) {
			continue
		}
		entries = append(entries, InterfaceEntry{Name: name, IsIB: false})
	}

	ibDir := "/sys/class/infiniband"
	ibEntries, err := os.ReadDir(ibDir)
	if err != nil {
		logrus.WithField("component", "transceiver").Debugf("no infiniband devices: %v", err)
		return entries, nil
	}
	for _, e := range ibEntries {
		ibDev := e.Name()
		physfn := filepath.Join(ibDir, ibDev, "device", "physfn")
		if _, err := os.Stat(physfn); err == nil {
			continue
		}
		netDevDir := filepath.Join(ibDir, ibDev, "device", "net")
		netDevs, _ := os.ReadDir(netDevDir)
		netDevName := ""
		if len(netDevs) > 0 {
			netDevName = netDevs[0].Name()
		}
		entries = append(entries, InterfaceEntry{Name: netDevName, IsIB: true, IBDev: ibDev})
	}

	return entries, nil
}

type NetworkClassifier struct {
	patterns map[string][]string
}

func NewNetworkClassifier(patterns map[string][]string) *NetworkClassifier {
	return &NetworkClassifier{patterns: patterns}
}

func (c *NetworkClassifier) Classify(ifaceName string) string {
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

// GetLinkSpeed reads the interface link speed from sysfs (e.g. "25000" Mbps) and returns a human-readable string.
func GetLinkSpeed(ifaceName string) string {
	if ifaceName == "" {
		return ""
	}
	speedPath := filepath.Join("/sys/class/net", ifaceName, "speed")
	data, err := os.ReadFile(speedPath)
	if err != nil {
		return ""
	}
	speedStr := strings.TrimSpace(string(data))
	if speedStr == "" || speedStr == "-1" {
		return ""
	}
	return speedStr + " Mbps"
}
