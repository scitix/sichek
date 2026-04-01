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
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type L1Checker struct {
	spec        *config.EthernetSpecConfig
	prevCRC     map[string]int64
	prevCarrier map[string]int64
	prevDrops   map[string]int64
}

func (c *L1Checker) Name() string { return config.EthernetL1CheckerName }

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
				logrus.WithFields(logrus.Fields{
					"checker": c.Name(),
					"nic":     slaveName,
				}).Errorf("Physical NIC link not UP")
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "LinkDown"
				result.Detail += fmt.Sprintf("Physical NIC %s link not UP. Command: ethtool %s, Expected: Link detected: yes, Actual: not connected or unknown.\n", slaveName, slaveName)
			}

			if len(info.SyslogErrors) > 0 {
				for _, errLine := range info.SyslogErrors {
					if strings.Contains(errLine, "tx timeout") && strings.Contains(errLine, slaveName) {
						logrus.WithFields(logrus.Fields{
							"checker": c.Name(),
							"nic":     slaveName,
							"msg":     errLine,
						}).Errorf("NIC tx timeout found in kernel log")
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
				logrus.WithFields(logrus.Fields{
					"checker":  c.Name(),
					"nic":      slaveName,
					"curr":     speedStr,
					"expected": expectedSpeed,
				}).Errorf("NIC speed mismatch")
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
				logrus.WithFields(logrus.Fields{
					"checker": c.Name(),
					"nic":     slaveName,
					"prev":    prev,
					"curr":    currCRC,
				}).Errorf("NIC RX (CRC) errors increasing")
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "CRCErrorsGrowing"
				result.Detail += fmt.Sprintf("NIC %s RX (CRC) errors increasing. Command: ip -s link show %s, Previous: %d, Current: %d.\n", slaveName, slaveName, prev, currCRC)
			}
			c.prevCRC[slaveName] = currCRC

			// Carrier errors
			currCarrierIPS := sStats.Carrier
			if prev, ok := c.prevCarrier[slaveName]; ok && currCarrierIPS > prev {
				logrus.WithFields(logrus.Fields{
					"checker": c.Name(),
					"nic":     slaveName,
					"prev":    prev,
					"curr":    currCarrierIPS,
				}).Errorf("NIC Carrier errors increasing")
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "CarrierErrorsGrowing"
				result.Detail += fmt.Sprintf("NIC %s Carrier errors increasing. Command: ip -s link show %s, Previous: %d, Current: %d.\n", slaveName, slaveName, prev, currCarrierIPS)
			}
			c.prevCarrier[slaveName] = currCarrierIPS

			// Drops
			currDrops := sStats.Dropped
			if prev, ok := c.prevDrops[slaveName]; ok && currDrops > prev {
				logrus.WithFields(logrus.Fields{
					"checker": c.Name(),
					"nic":     slaveName,
					"prev":    prev,
					"curr":    currDrops,
				}).Errorf("NIC Drops increasing")
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
