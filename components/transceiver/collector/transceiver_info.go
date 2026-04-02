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
	for _, iface := range interfaces {
		netType := c.networkClassifier.Classify(iface.Name)
		var module ModuleInfo

		if iface.IsIB {
			module, err = CollectMLXLink(ctx, iface.IBDev)
			module.Interface = iface.Name
			module.IBDev = iface.IBDev
			module.CollectTool = "mlxlink"
		} else {
			module, err = CollectEthtool(ctx, iface.Name)
			module.Interface = iface.Name
			module.CollectTool = "ethtool"
		}
		if err != nil {
			logrus.WithField("component", "transceiver").Warnf("collect %s failed: %v", iface.Name, err)
			module.Present = false
		}
		module.NetworkType = netType
		info.Modules = append(info.Modules, module)
	}

	return info, nil
}
