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
		logrus.WithField("checker", c.Name()).Errorf("FAILED/INCOMPLETE entries found in ARP neighbor table: %s", info.IPNeigh)
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.ErrorName = "ARPFailed"
		result.Detail = "FAILED/INCOMPLETE entries found in ARP neighbor table. Command: ip neigh show, L2 MAC resolution failed for some neighbors."
		result.Suggestion = "Verify local and switch VLAN ID config and L2 forwarding, or use arping to test connectivity and tcpdump for ARP."
	}

	if info.Routes.GatewayIP != "" && !info.Routes.GatewayReachable {
		logrus.WithFields(logrus.Fields{
			"checker": c.Name(),
			"gateway_ip": info.Routes.GatewayIP,
		}).Errorf("System gateway unreachable")
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.ErrorName = "GatewayUnreachable"
		result.Detail += fmt.Sprintf("System gateway (%s) unreachable. Command: ping -c 3 %s && ip neigh show %s.\n", info.Routes.GatewayIP, info.Routes.GatewayIP, info.Routes.GatewayIP)
	}

	return result, nil
}
