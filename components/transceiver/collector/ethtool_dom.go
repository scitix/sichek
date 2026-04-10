package collector

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

func CollectEthtool(ctx context.Context, netdev string) (ModuleInfo, error) {
	module := ModuleInfo{Present: true}

	cmdCtx, cancel := context.WithTimeout(ctx, consts.CmdTimeout)
	defer cancel()

	output, err := utils.ExecCommand(cmdCtx, "ethtool", "-m", netdev)
	if err != nil {
		return ModuleInfo{Present: false}, err
	}

	module.parseEthtoolModule(string(output))
	module.parseLinkErrors(ctx, netdev)
	return module, nil
}

func (m *ModuleInfo) parseEthtoolModule(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch {
		case key == "Identifier" || key == "Transceiver type":
			m.ModuleType = val
		case key == "Vendor name":
			m.Vendor = val
		case key == "Vendor PN":
			m.PartNumber = val
		case key == "Vendor SN":
			m.SerialNumber = val
		case key == "Module temperature high alarm threshold":
			m.TempHighAlarm = parseFloat(val)
		case key == "Module temperature low alarm threshold":
			m.TempLowAlarm = parseFloat(val)
		case key == "Module temperature":
			m.Temperature = parseFloat(val)
		case key == "Module voltage high alarm threshold":
			m.VoltageHighAlarm = parseFloat(val)
		case key == "Module voltage low alarm threshold":
			m.VoltageLowAlarm = parseFloat(val)
		case key == "Module voltage":
			m.Voltage = parseFloat(val)
		// Alarm thresholds (exact match) MUST come before HasPrefix matches
		case key == "Laser tx power high alarm threshold":
			m.TxPowerHighAlarm = parseDBM(val)
		case key == "Laser tx power low alarm threshold":
			m.TxPowerLowAlarm = parseDBM(val)
		case key == "Laser rx power high alarm threshold":
			m.RxPowerHighAlarm = parseDBM(val)
		case key == "Laser rx power low alarm threshold":
			m.RxPowerLowAlarm = parseDBM(val)
		// Prefix matches for per-lane values (after alarm thresholds)
		case strings.HasPrefix(key, "Laser tx power"):
			m.TxPower = append(m.TxPower, parseDBM(val))
		case strings.HasPrefix(key, "Receiver signal average optical power"):
			m.RxPower = append(m.RxPower, parseDBM(val))
		case strings.HasPrefix(key, "Laser tx bias current"):
			m.BiasCurrent = append(m.BiasCurrent, parseFloat(val))
		}
	}
}

func (m *ModuleInfo) parseLinkErrors(ctx context.Context, netdev string) {
	cmdCtx, cancel := context.WithTimeout(ctx, consts.CmdTimeout)
	defer cancel()

	output, err := utils.ExecCommand(cmdCtx, "ethtool", "-S", netdev)
	if err != nil {
		logrus.WithField("component", "transceiver").Debugf("ethtool -S %s failed: %v", netdev, err)
		return
	}

	m.LinkErrors = make(map[string]uint64)
	errorKeys := []string{"rx_crc_errors", "rx_fcs_errors", "rx_errors", "tx_errors"}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, key := range errorKeys {
			if strings.HasPrefix(line, key+":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					val, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
					m.LinkErrors[key] = val
				}
			}
		}
	}
}

var floatRegex = regexp.MustCompile(`[-+]?[0-9]*\.?[0-9]+`)

func parseFloat(s string) float64 {
	match := floatRegex.FindString(s)
	if match == "" {
		return 0
	}
	val, _ := strconv.ParseFloat(match, 64)
	return val
}

func parseDBM(s string) float64 {
	if idx := strings.Index(s, "dBm"); idx > 0 {
		part := strings.TrimSpace(s[:idx])
		if slashIdx := strings.LastIndex(part, "/"); slashIdx > 0 {
			part = strings.TrimSpace(part[slashIdx+1:])
		}
		val, _ := strconv.ParseFloat(strings.TrimSpace(part), 64)
		return val
	}
	return parseFloat(s)
}

