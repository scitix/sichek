package collector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

type TransceiverInfo struct {
	Modules []ModuleInfo `json:"modules"`
}

func (t *TransceiverInfo) JSON() (string, error) {
	data, err := json.MarshalIndent(t, "", "  ")
	return string(data), err
}

type ModuleInfo struct {
	Interface    string `json:"interface"`
	IBDev        string `json:"ib_dev"`
	NetworkType  string `json:"network_type"`
	CollectTool  string `json:"collect_tool"`

	Present      bool   `json:"present"`
	ModuleType   string `json:"module_type"`
	Vendor       string `json:"vendor"`
	PartNumber   string `json:"part_number"`
	SerialNumber string `json:"serial_number"`
	LinkSpeed    string `json:"link_speed"`

	Temperature float64   `json:"temperature_c"`
	Voltage     float64   `json:"voltage_v"`
	TxPower     []float64 `json:"tx_power_dbm"`
	RxPower     []float64 `json:"rx_power_dbm"`
	BiasCurrent []float64 `json:"bias_current_ma"`

	TxPowerHighAlarm float64 `json:"tx_power_high_alarm_dbm"`
	TxPowerLowAlarm  float64 `json:"tx_power_low_alarm_dbm"`
	RxPowerHighAlarm float64 `json:"rx_power_high_alarm_dbm"`
	RxPowerLowAlarm  float64 `json:"rx_power_low_alarm_dbm"`
	TempHighAlarm    float64 `json:"temp_high_alarm_c"`
	TempLowAlarm     float64 `json:"temp_low_alarm_c"`
	VoltageHighAlarm float64 `json:"voltage_high_alarm_v"`
	VoltageLowAlarm  float64 `json:"voltage_low_alarm_v"`

	LinkErrors map[string]uint64 `json:"link_errors"`
}

type TransceiverCollector struct {
	networkClassifier *NetworkClassifier
}

func NewTransceiverCollector(classifier *NetworkClassifier) *TransceiverCollector {
	return &TransceiverCollector{networkClassifier: classifier}
}

func (c *TransceiverCollector) Name() string {
	return "TransceiverCollector"
}

func (c *TransceiverCollector) Collect(ctx context.Context) (*TransceiverInfo, error) {
	interfaces, err := EnumerateTransceiverInterfaces()
	if err != nil {
		return nil, fmt.Errorf("enumerate interfaces failed: %w", err)
	}

	info := &TransceiverInfo{}
	seen := make(map[string]bool) // deduplicate by interface name

	// IB interfaces first (they have richer data via mlxlink)
	for _, iface := range interfaces {
		if !iface.IsIB {
			continue
		}
		if iface.Name != "" {
			seen[iface.Name] = true
		}
		module, collectErr := CollectMLXLink(ctx, iface.IBDev)
		if collectErr != nil {
			logrus.WithField("component", "transceiver").Debugf("skip IB %s: %v", iface.IBDev, collectErr)
			continue
		}
		module.Interface = iface.Name
		module.IBDev = iface.IBDev
		module.CollectTool = "mlxlink"
		module.NetworkType = c.networkClassifier.Classify(iface.Name)
		module.LinkSpeed = GetLinkSpeed(iface.Name)
		info.Modules = append(info.Modules, module)
	}

	// Then ethernet interfaces (skip if already collected via IB)
	for _, iface := range interfaces {
		if iface.IsIB {
			continue
		}
		if seen[iface.Name] {
			continue
		}

		var module ModuleInfo
		var collectErr error

		// mlx5 ethernet ports: use mlxlink with PCIe BDF (ethtool -m gives hex dump)
		if iface.IsMLX5 && iface.PcieBDF != "" {
			module, collectErr = CollectMLXLink(ctx, iface.PcieBDF)
			if collectErr != nil {
				logrus.WithField("component", "transceiver").Debugf("skip mlx5 %s (BDF %s): %v", iface.Name, iface.PcieBDF, collectErr)
				continue
			}
			module.CollectTool = "mlxlink"
		} else {
			module, collectErr = CollectEthtool(ctx, iface.Name)
			if collectErr != nil {
				logrus.WithField("component", "transceiver").Debugf("skip %s: no transceiver module detected (%v)", iface.Name, collectErr)
				continue
			}
			module.CollectTool = "ethtool"
		}

		module.Interface = iface.Name
		module.NetworkType = c.networkClassifier.Classify(iface.Name)
		module.LinkSpeed = GetLinkSpeed(iface.Name)
		info.Modules = append(info.Modules, module)
	}

	return info, nil
}
