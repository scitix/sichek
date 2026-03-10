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
package checker

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/consts"
)

func NewCheckers(cfg *config.EthernetUserConfig, spec *config.EthernetSpecConfig) ([]common.Checker, error) {
	checkers := []common.Checker{
		&L1Checker{
			spec:        spec,
			prevCRC:     make(map[string]int64),
			prevCarrier: make(map[string]int64),
			prevDrops:   make(map[string]int64),
		},
		&L2Checker{
			spec:             spec,
			prevLinkFailures: make(map[string]int64),
			prevActiveSlave:  make(map[string]string),
		},
		&L3Checker{spec: spec},
		&L4Checker{spec: spec},
		&L5Checker{spec: spec},
	}
	// Filter skipped checkers
	ignoredMap := make(map[string]bool)
	if cfg != nil && cfg.Ethernet != nil {
		for _, v := range cfg.Ethernet.IgnoredCheckers {
			ignoredMap[v] = true
		}
	}
	var activeCheckers []common.Checker
	for _, chk := range checkers {
		if !ignoredMap[chk.Name()] {
			activeCheckers = append(activeCheckers, chk)
		}
	}
	return activeCheckers, nil
}

type L1Checker struct {
	spec        *config.EthernetSpecConfig
	prevCRC     map[string]int64
	prevCarrier map[string]int64
	prevDrops   map[string]int64
}

func (c *L1Checker) Name() string { return config.EthernetL1CheckerName }

// extractInt parses an integer using regex from a string pattern
func extractInt(input, pattern string) int64 {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		val, _ := strconv.ParseInt(matches[1], 10, 64)
		return val
	}
	return 0
}

func (c *L1Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	expectedSpeed := "25000" // default to 25G
	if c.spec != nil && c.spec.Speed != "" {
		expectedSpeed = c.spec.Speed
	}

	for _, bond := range info.BondInterfaces {
		for slaveName, slaveState := range info.Slaves[bond] {
			if !slaveState.LinkDetected {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "LinkDown"
				result.Detail += fmt.Sprintf("Physical NIC %s link not UP. Command: ethtool %s, Expected: Link detected: yes, Actual: not connected or unknown.\n", slaveName, slaveName)
			}

			if len(info.SyslogErrors) > 0 {
				for _, errLine := range info.SyslogErrors {
					if strings.Contains(errLine, "tx timeout") && strings.Contains(errLine, slaveName) {
						result.Status = consts.StatusAbnormal
						result.Level = consts.LevelCritical
						result.ErrorName = "TxTimeout"
						result.Detail += fmt.Sprintf("NIC %s tx timeout found in kernel log. Command: dmesg | grep -iE 'eth|mlx|link'.\n", slaveName)
						break
					}
				}
			}

			// check speed
			speedStr := strconv.Itoa(slaveState.Speed)
			if speedStr != expectedSpeed && slaveState.Speed > 0 {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "SpeedMismatch"
				result.Detail += fmt.Sprintf("NIC %s speed mismatch. Command: ethtool %s, Expected: %sMb/s, Actual: %sMb/s.\n", slaveName, slaveName, expectedSpeed, speedStr)
			}

			// Parse stats
			sStats := info.Stats[slaveName]

			// CRC errors
			currCRC := sStats.RXErrors // Approximation, standard ip -s link maps CRC errors to RX errors broadly. For exact CRC, ethtool parsing should remain, but for now we follow the general RX error growth.
			if prev, ok := c.prevCRC[slaveName]; ok && currCRC > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "CRCErrorsGrowing"
				result.Detail += fmt.Sprintf("NIC %s RX (CRC) errors increasing. Command: ip -s link show %s, Previous: %d, Current: %d.\n", slaveName, slaveName, prev, currCRC)
			}
			c.prevCRC[slaveName] = currCRC

			// Carrier errors
			currCarrierIPS := sStats.Carrier
			if prev, ok := c.prevCarrier[slaveName]; ok && currCarrierIPS > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "CarrierErrorsGrowing"
				result.Detail += fmt.Sprintf("NIC %s Carrier errors increasing. Command: ip -s link show %s, Previous: %d, Current: %d.\n", slaveName, slaveName, prev, currCarrierIPS)
			}
			c.prevCarrier[slaveName] = currCarrierIPS

			// Drops
			currDrops := sStats.Dropped
			if prev, ok := c.prevDrops[slaveName]; ok && currDrops > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "DropsGrowing"
				result.Detail += fmt.Sprintf("NIC %s Drops increasing. Command: ip -s link show %s, Previous: %d, Current: %d.\n", slaveName, slaveName, prev, currDrops)
			}
			c.prevDrops[slaveName] = currDrops
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "Please check physical link, cable, driver version (ethtool -i), or check dmesg for specific errors; if speed mismatch, check corresponding configuration."
	}

	return result, nil
}

type L2Checker struct {
	spec             *config.EthernetSpecConfig
	prevLinkFailures map[string]int64
	prevActiveSlave  map[string]string
}

func (c *L2Checker) Name() string { return config.EthernetL2CheckerName }
func (c *L2Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	expectedMinSlaves := 2
	if c.spec != nil && c.spec.MinSlaves > 0 {
		expectedMinSlaves = c.spec.MinSlaves
	}

	for _, bond := range info.BondInterfaces {
		bState, exists := info.Bonds[bond]
		if !exists || bState.Name == "" {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "BondingMissing"
			result.Detail = fmt.Sprintf("Bond %s missing in /proc/net/bonding. Command: ls /proc/net/bonding/.\n", bond)
			continue
		}

		expectedMII := "up"
		procContent := info.ProcNetBonding[bond]
		if c.spec != nil && c.spec.MIIStatus != "" {
			expectedMII = c.spec.MIIStatus
		}

		if (expectedMII == "up" && !bState.IsUp) || !strings.Contains(procContent, "MII Status: "+expectedMII) {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "BondDown"
			result.Detail += fmt.Sprintf("Overall status of bond interface %s mismatch. Command: cat /proc/net/bonding/%s, Expected: MII Status: %s, Actual: mismatch (possibly down).\n", bond, bond, expectedMII)
		}

		// check MTU
		if c.spec != nil && c.spec.MTU != "" {
			expectedMTU, _ := strconv.Atoi(c.spec.MTU)
			if bState.MTU > 0 && bState.MTU != expectedMTU {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "MTUMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s MTU mismatch. Command: ip link show %s, Expected: %d, Actual: %d.\n", bond, bond, expectedMTU, bState.MTU)
			}
		}

		// check xmit_hash_policy
		if c.spec != nil && c.spec.XmitHashPolicy != "" {
			if bState.XmitHashPolicy != "" && bState.XmitHashPolicy != c.spec.XmitHashPolicy {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "XmitHashPolicyMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s xmit_hash_policy mismatch. Command: cat /sys/class/net/%s/bonding/xmit_hash_policy, Expected: %s, Actual: %s.\n", bond, bond, c.spec.XmitHashPolicy, bState.XmitHashPolicy)
			}
		}

		// check slave count
		slaveCount := len(info.Slaves[bond])
		if slaveCount < expectedMinSlaves {
			if result.Status == consts.StatusNormal {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "SlaveCountMismatch"
			}
			result.Detail += fmt.Sprintf("Bond %s insufficient slave count. Command: cat /proc/net/bonding/%s, Expected at least: %d, Actual: %d.\n", bond, bond, expectedMinSlaves, slaveCount)
		}

		// check miimon, updelay, downdelay (fetching downdelay and updelay via regex since they aren't fully standard across systems on sysfs)
		miimon := int64(bState.Miimon)
		updelay := extractInt(procContent, `Up Delay \(ms\):\s*(\d+)`)
		downdelay := extractInt(procContent, `Down Delay \(ms\):\s*(\d+)`)

		expectedMiimon := int64(0)
		expectedUpDelay := int64(0)
		expectedDownDelay := int64(0)
		if c.spec != nil {
			if c.spec.Miimon > 0 {
				expectedMiimon = int64(c.spec.Miimon)
			}
			expectedUpDelay = int64(c.spec.UpDelay)
			expectedDownDelay = int64(c.spec.DownDelay)
		}

		if miimon == 0 {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "MiimonDisabled"
			result.Detail += fmt.Sprintf("Bond %s MII Polling Interval (miimon) is 0. Command: cat /proc/net/bonding/%s, please enable link detection (miimon) to avoid packet loss.\n", bond, bond)
		} else {
			if expectedMiimon > 0 && miimon != expectedMiimon {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "MiimonMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s miimon mismatch. Command: cat /sys/class/net/%s/bonding/miimon, Expected: %d ms, Actual: %d ms.\n", bond, bond, expectedMiimon, miimon)
			}

			if downdelay < miimon {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "DowndelayTooSmall"
				result.Detail += fmt.Sprintf("Bond %s downdelay (%d ms) less than miimon (%d ms). Command: cat /proc/net/bonding/%s, unreasonable config may cause flapping.\n", bond, downdelay, miimon, bond)
			} else if expectedDownDelay > 0 && downdelay != expectedDownDelay {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "DowndelayMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s downdelay mismatch. Command: cat /sys/class/net/%s/bonding/downdelay, Expected: %d ms, Actual: %d ms.\n", bond, bond, expectedDownDelay, downdelay)
			}

			if updelay == 0 {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "UpdelayZero"
				result.Detail += fmt.Sprintf("Bond %s updelay is 0. Command: cat /proc/net/bonding/%s, updelay is recommended to avoid packet loss during switch port negotiation.\n", bond, bond)
			} else if expectedUpDelay > 0 && updelay != expectedUpDelay {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "UpdelayMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s updelay mismatch. Command: cat /sys/class/net/%s/bonding/updelay, Expected: %d ms, Actual: %d ms.\n", bond, bond, expectedUpDelay, updelay)
			}
		}

		// track active slave for flapping detection
		activeSlaveMatch := regexp.MustCompile(`Currently Active Slave:\s*(\w+)`).FindStringSubmatch(procContent)
		if len(activeSlaveMatch) > 1 {
			currActive := activeSlaveMatch[1]
			if prev, ok := c.prevActiveSlave[bond]; ok && prev != "" && prev != currActive {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "ActiveSlaveFlapping"
				result.Detail += fmt.Sprintf("Bond %s active slave switched. Command: cat /proc/net/bonding/%s, Previous: %s, Current: %s. If frequent, please focus on physical layer stability.\n", bond, bond, prev, currActive)
			}
			c.prevActiveSlave[bond] = currActive
		}

		// track Link Failure Count per slave
		slavesData := strings.Split(procContent, "Slave Interface: ")
		for i := 1; i < len(slavesData); i++ {
			lines := strings.Split(slavesData[i], "\n")
			if len(lines) == 0 {
				continue
			}
			slaveName := strings.TrimSpace(lines[0])
			failCount := extractInt(slavesData[i], `Link Failure Count:\s*(\d+)`)
			trackKey := bond + "-" + slaveName

			if prev, ok := c.prevLinkFailures[trackKey]; ok && failCount > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "LinkFailureGrowing"
				result.Detail += fmt.Sprintf("Bond %s slave NIC %s link failure occurred. Command: cat /proc/net/bonding/%s, Previous count: %d, Current: %d.\n", bond, slaveName, bond, prev, failCount)
			}
			c.prevLinkFailures[trackKey] = failCount
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "Please use cat /proc/net/bonding/bond0 to verify MII status and Link Failure Count; ensure config (e.g., /etc/netplan) has miimon > 0."
	}

	return result, nil
}

type L3Checker struct{ spec *config.EthernetSpecConfig }

func (c *L3Checker) Name() string { return config.EthernetL3CheckerName }
func (c *L3Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, bond := range info.BondInterfaces {
		bState, ok := info.Bonds[bond]
		if !ok || !strings.Contains(bState.Mode, "802.3ad") {
			continue
		}

		lacp, exists := info.LACP[bond]
		if !exists || lacp.PartnerMacAddress == "" {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "ActiveAggregatorMissing"
			result.Detail += fmt.Sprintf("Bond %s configured as 802.3ad mode but no valid Active Aggregator found. Command: cat /proc/net/bonding/%s, peer switch might not have LACP configured or link is abnormal.\n", bond, bond)
			continue
		}

		if lacp.PartnerMacAddress == "00:00:00:00:00:00" {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "PartnerMacInvalid"
			result.Detail += fmt.Sprintf("Bond %s Partner Mac Address is all zeros. Command: cat /proc/net/bonding/%s, peer switch did not respond to LACP packets.\n", bond, bond)
		}

		for slaveName, sState := range info.Slaves[bond] {
			if !sState.IsUp {
				continue
			}

			if slaveAggID, ok := lacp.SlaveAggregatorIDs[slaveName]; ok && slaveAggID != lacp.ActiveAggregatorID {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "AggregatorMismatch"
				result.Detail += fmt.Sprintf("Slave NIC %s Aggregator ID (%s) mismatch with global Active Aggregator ID (%s). Command: cat /proc/net/bonding/%s, it cannot join the aggregation group.\n", slaveName, slaveAggID, lacp.ActiveAggregatorID, bond)
			}

			if portKey, ok := lacp.SlaveActorKeys[slaveName]; ok && portKey != lacp.ActorKey {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "ActorKeyMismatch"
				result.Detail += fmt.Sprintf("Slave NIC %s port key (%s) mismatch with global Actor Key (%s). Command: cat /proc/net/bonding/%s.\n", slaveName, portKey, lacp.ActorKey, bond)
			}

			if operKey, ok := lacp.SlavePartnerKeys[slaveName]; ok && operKey != lacp.PartnerKey {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "PartnerKeyMismatch"
				result.Detail += fmt.Sprintf("Slave NIC %s oper key (%s) mismatch with global Partner Key (%s). Command: cat /proc/net/bonding/%s, peer LACP negotiation abnormal.\n", slaveName, operKey, lacp.PartnerKey, bond)
			}
		}

		if c.spec != nil && c.spec.LACPRate != "" {
			if !strings.Contains(bState.LACPRate, c.spec.LACPRate) {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "LACPRateMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s LACP rate mismatch. Command: cat /sys/class/net/%s/bonding/lacp_rate, Expected: %s, Actual: %s.\n", bond, bond, c.spec.LACPRate, bState.LACPRate)
			}
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "Recommended to simultaneously troubleshoot LACP / Eth-Trunk aggregation config on peer switch."
	}

	return result, nil
}

type L4Checker struct{ spec *config.EthernetSpecConfig }

func (c *L4Checker) Name() string { return config.EthernetL4CheckerName }
func (c *L4Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	if strings.Contains(info.IPNeigh, "FAILED") || strings.Contains(info.IPNeigh, "INCOMPLETE") {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.ErrorName = "ARPFailed"
		result.Detail = "FAILED/INCOMPLETE entries found in ARP neighbor table. Command: ip neigh show, L2 MAC resolution failed for some neighbors."
		result.Suggestion = "Verify local and switch VLAN ID config and L2 forwarding, or use arping to test connectivity and tcpdump for ARP."
	}

	if info.Routes.GatewayIP != "" && !info.Routes.GatewayReachable {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.ErrorName = "GatewayUnreachable"
		result.Detail += fmt.Sprintf("System gateway (%s) unreachable. Command: ping -c 3 %s && ip neigh show %s.\n", info.Routes.GatewayIP, info.Routes.GatewayIP, info.Routes.GatewayIP)
	}

	return result, nil
}

type L5Checker struct{ spec *config.EthernetSpecConfig }

func (c *L5Checker) Name() string { return config.EthernetL5CheckerName }
func (c *L5Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	if !info.Routes.DefaultRouteViaBond {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.ErrorName = "DirectRouteMismatch"
		result.Detail += "System default route does not point directly to target bond. Command: ip route show default, business traffic might not use bond.\n"
	}

	if info.RPFilter["all"] == "1" {
		if result.Status == consts.StatusNormal {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelWarning
			result.ErrorName = "RPFilterEnabled"
		}
		result.Detail += "System enabled rp_filter (all=1). Command: sysctl -n net.ipv4.conf.all.rp_filter, Expected: 0 or 2, Actual: 1.\n"
	}

	for bond, val := range info.RPFilter {
		if bond != "all" && val == "1" {
			if result.Status == consts.StatusNormal {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "RPFilterEnabled"
			}
			result.Detail += fmt.Sprintf("Bond %s enabled rp_filter=1. Command: sysctl -n net.ipv4.conf.%s.rp_filter, Expected: 0 or 2, Actual: 1.\n", bond, bond)
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "If packet loss occurs, it is recommended to check route matching, policy routing (ip rule), and set rp_filter to 0 or 2."
	}

	return result, nil
}
