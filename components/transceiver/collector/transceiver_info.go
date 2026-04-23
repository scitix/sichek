package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

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
	Interface   string `json:"interface"`
	IBDev       string `json:"ib_dev"`
	NetworkType string `json:"network_type"`
	CollectTool string `json:"collect_tool"`

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

type collectTask struct {
	iface      InterfaceEntry
	useMLXLink bool
	dev        string // IBDev name, PCIe BDF, or interface name
}

type indexedModule struct {
	idx    int
	module ModuleInfo
	iface  InterfaceEntry
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

	// Build the deduplicated task list: IB entries take priority over Ethernet entries
	// for the same interface name (mlxlink via IBDev gives richer data).
	// BDF-based deduplication prevents collecting VF-rep interfaces that share
	// the same physical PCIe function as their parent PF.
	seenName := make(map[string]bool)
	seenBDF := make(map[string]bool)
	var tasks []collectTask

	for _, iface := range interfaces {
		if !iface.IsIB {
			continue
		}
		if iface.Name != "" {
			seenName[iface.Name] = true
		}
		if iface.PcieBDF != "" {
			seenBDF[iface.PcieBDF] = true
		}
		tasks = append(tasks, collectTask{iface: iface, useMLXLink: true, dev: iface.IBDev})
	}

	for _, iface := range interfaces {
		if iface.IsIB || seenName[iface.Name] {
			continue
		}
		if iface.PcieBDF != "" && seenBDF[iface.PcieBDF] {
			logrus.WithField("component", "transceiver").Debugf("skip %s: same BDF %s as IB interface", iface.Name, iface.PcieBDF)
			continue
		}
		if iface.IsMLX5 && iface.PcieBDF != "" {
			tasks = append(tasks, collectTask{iface: iface, useMLXLink: true, dev: iface.PcieBDF})
		} else {
			tasks = append(tasks, collectTask{iface: iface, useMLXLink: false, dev: iface.Name})
		}
	}

	// Collect all interfaces concurrently to avoid sequential mlxlink timeouts.
	resultCh := make(chan indexedModule, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(i int, task collectTask) {
			defer wg.Done()

			var module ModuleInfo
			var collectErr error

			if task.useMLXLink {
				module, collectErr = CollectMLXLink(ctx, task.dev)
				if collectErr != nil {
					if task.iface.IsIB {
						logrus.WithField("component", "transceiver").Debugf("skip IB %s: %v", task.dev, collectErr)
					} else {
						logrus.WithField("component", "transceiver").Debugf("skip mlx5 %s (BDF %s): %v", task.iface.Name, task.dev, collectErr)
					}
					return
				}
				module.CollectTool = "mlxlink"
			} else {
				module, collectErr = CollectEthtool(ctx, task.dev)
				if collectErr != nil {
					logrus.WithField("component", "transceiver").Debugf("skip %s: no transceiver module detected (%v)", task.iface.Name, collectErr)
					return
				}
				module.CollectTool = "ethtool"
			}

			module.Interface = task.iface.Name
			if task.iface.IsIB {
				module.IBDev = task.iface.IBDev
			}

			resultCh <- indexedModule{idx: i, module: module, iface: task.iface}
		}(i, task)
	}

	wg.Wait()
	close(resultCh)

	// Preserve original order (IB before Ethernet, as tasks were ordered).
	ordered := make([]indexedModule, 0, len(tasks))
	for r := range resultCh {
		ordered = append(ordered, r)
	}
	sortByIndex(ordered)

	info := &TransceiverInfo{}
	for _, r := range ordered {
		module := r.module
		speedStr, speedMbps := GetLinkSpeed(r.iface.Name)
		module.LinkSpeed = speedStr
		module.NetworkType = c.networkClassifier.Classify(r.iface.Name, speedMbps)
		info.Modules = append(info.Modules, module)
	}

	return info, nil
}

func sortByIndex(items []indexedModule) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].idx < items[j-1].idx; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
