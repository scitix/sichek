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
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

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

	if len(info.BondInterfaces) == 0 {
		logrus.WithField("checker", c.Name()).Errorf("No bond interfaces found")
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.ErrorName = "NoBondInterface"
		result.Detail = "No bond interfaces found. Command: ls /proc/net/bonding/."
		result.Suggestion = "Please check if bond interfaces are configured correctly, e.g., /etc/netplan or /etc/sysconfig/network-scripts."
		return result, nil
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
		bState, ok := info.Bonds[bond]
		if !ok || !strings.Contains(bState.Mode, "802.3ad") {
			continue
		}

		lacp, exists := info.LACP[bond]
		if !exists || lacp.PartnerMacAddress == "" {
			logrus.WithFields(logrus.Fields{
				"checker": c.Name(),
				"bond":    bond,
			}).Errorf("Active Aggregator missing for 802.3ad bond")
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "ActiveAggregatorMissing"
			result.Detail += fmt.Sprintf("Bond %s configured as 802.3ad mode but no valid Active Aggregator found. Command: cat /proc/net/bonding/%s, peer switch might not have LACP configured or link is abnormal.\n", bond, bond)
			continue
		}

		if lacp.PartnerMacAddress == "00:00:00:00:00:00" {
			logrus.WithFields(logrus.Fields{
				"checker": c.Name(),
				"bond":    bond,
				"partner_mac": lacp.PartnerMacAddress,
			}).Errorf("Partner Mac Address is invalid (zeros)")
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
				logrus.WithFields(logrus.Fields{
					"checker":    c.Name(),
					"bond":       bond,
					"nic":        slaveName,
					"slave_agg":  slaveAggID,
					"active_agg": lacp.ActiveAggregatorID,
				}).Errorf("Aggregator ID mismatch")
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "AggregatorMismatch"
				result.Detail += fmt.Sprintf("Slave NIC %s Aggregator ID (%s) mismatch with global Active Aggregator ID (%s). Command: cat /proc/net/bonding/%s, it cannot join the aggregation group.\n", slaveName, slaveAggID, lacp.ActiveAggregatorID, bond)
			}

			if portKey, ok := lacp.SlaveActorKeys[slaveName]; ok && portKey != lacp.ActorKey {
				logrus.WithFields(logrus.Fields{
					"checker":   c.Name(),
					"bond":      bond,
					"nic":       slaveName,
					"port_key":  portKey,
					"actor_key": lacp.ActorKey,
				}).Errorf("Actor Key mismatch")
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "ActorKeyMismatch"
				result.Detail += fmt.Sprintf("Slave NIC %s port key (%s) mismatch with global Actor Key (%s). Command: cat /proc/net/bonding/%s.\n", slaveName, portKey, lacp.ActorKey, bond)
			}

			if operKey, ok := lacp.SlavePartnerKeys[slaveName]; ok && operKey != lacp.PartnerKey {
				logrus.WithFields(logrus.Fields{
					"checker":     c.Name(),
					"bond":        bond,
					"nic":         slaveName,
					"oper_key":    operKey,
					"partner_key": lacp.PartnerKey,
				}).Errorf("Partner Key mismatch")
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "PartnerKeyMismatch"
				result.Detail += fmt.Sprintf("Slave NIC %s oper key (%s) mismatch with global Partner Key (%s). Command: cat /proc/net/bonding/%s, peer LACP negotiation abnormal.\n", slaveName, operKey, lacp.PartnerKey, bond)
			}
		}

		if c.spec != nil && c.spec.LACPRate != "" {
			if !strings.Contains(bState.LACPRate, c.spec.LACPRate) {
				logrus.WithFields(logrus.Fields{
					"checker":  c.Name(),
					"bond":     bond,
					"curr":     bState.LACPRate,
					"expected": c.spec.LACPRate,
				}).Errorf("LACP rate mismatch")
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
