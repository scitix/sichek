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

	if len(info.BondInterfaces) == 0 {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.ErrorName = "NoBondInterface"
		result.Detail = "No bond interfaces found. Command: ls /proc/net/bonding/."
		result.Suggestion = "Please check if bond interfaces are configured correctly, e.g., /etc/netplan or /etc/sysconfig/network-scripts."
		return result, nil
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
