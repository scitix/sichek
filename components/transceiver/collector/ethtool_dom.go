package collector

import (
	"context"
	"errors"
	"fmt"
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
	// ethtool can exit 0 yet decode only the identity page (or nothing but a
	// raw hex dump). Return what was decoded together with errNoDOM: mlx5 ports
	// fall back to mlxlink for the readings, other NICs keep the identity row.
	if !module.hasUsableDOM() {
		return module, fmt.Errorf("ethtool -m %s: %w", netdev, errNoDOM)
	}

	linkCtx, linkCancel := context.WithTimeout(ctx, consts.CmdTimeout)
	defer linkCancel()
	if out, err := utils.ExecCommand(linkCtx, "ethtool", netdev); err == nil {
		module.LinkLanes = parseEthtoolLinkLanes(string(out))
	}

	statsCtx, statsCancel := context.WithTimeout(ctx, consts.CmdTimeout)
	defer statsCancel()
	stats, err := utils.ExecCommand(statsCtx, "ethtool", "-S", netdev)
	if err != nil {
		logrus.WithField("component", "transceiver").Debugf("ethtool -S %s failed: %v", netdev, err)
	} else {
		module.parseEthtoolStats(string(stats))
	}
	return module, nil
}

// errNoDOM marks an `ethtool -m` result that decoded no diagnostic readings.
var errNoDOM = errors.New("no decodable DOM data")

// parseEthtoolLinkLanes returns the "Lanes:" value of `ethtool <if>`, or 0 when
// the link is down or the kernel does not report it.
func parseEthtoolLinkLanes(output string) int {
	for _, line := range strings.Split(output, "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || key != "Lanes" {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

// hasUsableDOM reports whether parseEthtoolModule decoded diagnostic readings.
// Vendor fields alone are not enough: mlx5 often serves only the SFP identity
// page to ethtool, leaving no power or temperature, while mlxlink still has them.
func (m *ModuleInfo) hasUsableDOM() bool {
	return len(m.RxPower) > 0 || len(m.TxPower) > 0 || m.Temperature != 0
}

// parseEthtoolModule parses `ethtool -m` output for SFF-8472 (SFP), SFF-8636
// (QSFP) and CMIS (QSFP-DD/OSFP) modules. The three memory maps label the same
// quantities differently, e.g. per-lane Tx power is "Laser output power" (SFP),
// "Transmit avg optical power (Channel N)" (QSFP/CMIS) and the Tx threshold is
// "Laser output power ... threshold" under CMIS.
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
		lower := strings.ToLower(key)

		// Thresholds first: they share prefixes with the live readings.
		if strings.HasSuffix(lower, "alarm threshold") {
			m.parseEthtoolThreshold(lower, val)
			continue
		}
		// Per-lane / per-module alarm and warning *flags* (e.g. "Laser tx power
		// high alarm (Channel 1): Off") and warning thresholds are not readings;
		// letting them through would parse "Off" as a 0 dBm power value.
		if strings.Contains(lower, "alarm") || strings.Contains(lower, "warning") {
			continue
		}

		switch {
		case key == "Identifier" || key == "Transceiver type":
			if m.ModuleType == "" || key == "Identifier" {
				m.ModuleType = val
			}
		case key == "Vendor name":
			m.Vendor = val
		case key == "Vendor PN":
			m.PartNumber = val
		case key == "Vendor SN":
			m.SerialNumber = val
		case key == "Module temperature":
			m.Temperature = parseFloat(val)
		case key == "Module voltage":
			m.Voltage = parseFloat(val)
		case strings.HasPrefix(lower, "laser tx power"),
			strings.HasPrefix(lower, "laser output power"),
			strings.HasPrefix(lower, "transmit avg optical power"):
			m.TxPower = append(m.TxPower, parseDBM(val))
		case strings.HasPrefix(lower, "receiver signal average optical power"),
			strings.HasPrefix(lower, "rcvr signal avg optical power"):
			m.RxPower = append(m.RxPower, parseDBM(val))
		case strings.HasPrefix(lower, "laser tx bias current"),
			strings.HasPrefix(lower, "laser bias current"):
			m.BiasCurrent = append(m.BiasCurrent, parseFloat(val))
		}
	}
}

func (m *ModuleInfo) parseEthtoolThreshold(lowerKey, val string) {
	high := strings.Contains(lowerKey, "high alarm")
	low := strings.Contains(lowerKey, "low alarm")
	if !high && !low {
		return
	}
	switch {
	case strings.HasPrefix(lowerKey, "module temperature"):
		if high {
			m.TempHighAlarm = parseFloat(val)
		} else {
			m.TempLowAlarm = parseFloat(val)
		}
	case strings.HasPrefix(lowerKey, "module voltage"):
		if high {
			m.VoltageHighAlarm = parseFloat(val)
		} else {
			m.VoltageLowAlarm = parseFloat(val)
		}
	case strings.HasPrefix(lowerKey, "laser tx power"), strings.HasPrefix(lowerKey, "laser output power"):
		if high {
			m.TxPowerHighAlarm = parseDBM(val)
		} else {
			m.TxPowerLowAlarm = parseDBM(val)
		}
	case strings.HasPrefix(lowerKey, "laser rx power"):
		if high {
			m.RxPowerHighAlarm = parseDBM(val)
		} else {
			m.RxPowerLowAlarm = parseDBM(val)
		}
	}
}

// parseEthtoolStats extracts link-error and physical-layer counters from
// `ethtool -S`. The mlx5 *_phy counters are the same PPCNT counters mlxlink -c
// reports, so the ethtool path keeps the counter metrics without an mlxlink call.
// BER *ratios* are left zero here: they need a bit count that is not exported on
// most kernels, and a cumulative counter alone cannot give a rate.
func (m *ModuleInfo) parseEthtoolStats(output string) {
	m.LinkErrors = make(map[string]uint64)
	errorKeys := map[string]bool{"rx_crc_errors": true, "rx_fcs_errors": true, "rx_errors": true, "tx_errors": true}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}
		if errorKeys[key] {
			m.LinkErrors[key] = val
			continue
		}
		switch key {
		case "rx_pcs_symbol_err_phy":
			m.EffectivePhysicalErrors = val
		case "rx_corrected_bits_phy":
			m.RawPhysicalErrors = val
		case "link_down_events_phy":
			m.LinkDownCounter = val
		case "total_success_recovery_phy":
			m.LinkErrorRecoveryCounter = val
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
		return finiteOrZero(val)
	}
	return parseFloat(s)
}
