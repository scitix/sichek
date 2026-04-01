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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

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
		logrus.WithField("checker", c.Name()).Errorf("System default route does not point directly to target bond")
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.ErrorName = "DirectRouteMismatch"
		result.Detail += "System default route does not point directly to target bond. Command: ip route show default, business traffic might not use bond.\n"
	}

	if info.RPFilter["all"] == "1" {
		logrus.WithFields(logrus.Fields{
			"checker": c.Name(),
			"interface": "all",
			"curr": "1",
		}).Errorf("System enabled rp_filter")
		if result.Status == consts.StatusNormal {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelWarning
			result.ErrorName = "RPFilterEnabled"
		}
		result.Detail += "System enabled rp_filter (all=1). Command: sysctl -n net.ipv4.conf.all.rp_filter, Expected: 0 or 2, Actual: 1.\n"
	}

	for bond, val := range info.RPFilter {
		if bond != "all" && val == "1" {
			logrus.WithFields(logrus.Fields{
				"checker":   c.Name(),
				"interface": bond,
				"curr":      "1",
			}).Errorf("Interface enabled rp_filter=1")
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
