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
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type IBDevsChecker struct {
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBDevsChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBDevsChecker{
		name: config.CheckIBDevs,
		spec: specCfg,
	}, nil
}

func (c *IBDevsChecker) Name() string {
	return c.name
}

func (c *IBDevsChecker) Description() string {
	return c.description
}

func (c *IBDevsChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBDevsChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]

	var failedDevices []string
	var mismatchDetails []string
	infinibandInfo.RLock()
	// 1) Spec -> actual: missing or wrong mapping
	for expectedMlx5, expectedIb := range c.spec.IBPFDevs {
		// skip mezzanine card in check
		if strings.Contains(expectedMlx5, "mezz") {
			logrus.WithField("component", "infiniband").Debugf("skip mezzanine card %s in check", expectedMlx5)
			continue
		}
		actualIb, found := infinibandInfo.IBPFDevs[expectedMlx5]
		if !found {
			failedDevices = append(failedDevices, expectedMlx5)
			mismatchDetails = append(mismatchDetails, fmt.Sprintf("%s (missing)", expectedMlx5))
			logrus.WithField("component", "infiniband").Debugf("mismatch pair %s in check", expectedMlx5)
			continue
		}

		if actualIb != expectedIb {
			logrus.WithField("component", "infiniband").Debugf("mismatch pair %s -> %s (expected %s)", expectedMlx5, actualIb, expectedIb)
			failedDevices = append(failedDevices, expectedMlx5)
			mismatchDetails = append(mismatchDetails, fmt.Sprintf("%s -> %s (expected %s)", expectedMlx5, actualIb, expectedIb))
		}
	}
	// 2) Actual -> spec: extra devices not defined in spec (e.g. mlx5_7 in spec but actual shows mlx5_13_6209)
	for actualMlx5 := range infinibandInfo.IBPFDevs {
		if strings.Contains(actualMlx5, "mezz") {
			continue
		}
		if utils.IsLowSpeedIBBond(actualMlx5) {
			logrus.WithField("component", "infiniband").Debugf("skip management bond %s in check", actualMlx5)
			continue
		}
		if _, inSpec := c.spec.IBPFDevs[actualMlx5]; !inSpec {
			failedDevices = append(failedDevices, actualMlx5)
			mismatchDetails = append(mismatchDetails, fmt.Sprintf("%s (not in spec)", actualMlx5))
			logrus.WithField("component", "infiniband").Debugf("mismatch: actual device %s not defined in spec", actualMlx5)
		}
	}
	infinibandInfo.RUnlock()

	if len(failedDevices) > 0 {
		result.Status = consts.StatusAbnormal
		result.Device = strings.Join(failedDevices, ",")
		// Read IBPFDevs under lock protection
		infinibandInfo.RLock()
		ibpfDevsCopy := make(map[string]string)
		for k, v := range infinibandInfo.IBPFDevs {
			if strings.Contains(k, "mezz") {
				logrus.WithField("component", "infiniband").Debugf("skip mezzanine card %s in detail display", k)
				continue
			}
			if utils.IsLowSpeedIBBond(k) {
				logrus.WithField("component", "infiniband").Debugf("skip management bond %s in detail display", k)
				continue
			}
			ibpfDevsCopy[k] = v
		}
		infinibandInfo.RUnlock()

		// Use mismatchDetails for a more helpful detail message
		detailStr := strings.Join(mismatchDetails, "; ")
		logrus.WithFields(logrus.Fields{
			"checker":  c.Name(),
			"mismatch": detailStr,
		}).Errorf("Infiniband device mapping mismatch detected")
		result.Detail = fmt.Sprintf("Mismatched IB devices: %s. Current map: %v, Expected map: %v", detailStr, ibpfDevsCopy, c.spec.IBPFDevs)
	} else {
		result.Status = consts.StatusNormal
	}
	var specSlice []string
	for mlxDev, ibDev := range c.spec.IBPFDevs {
		specSlice = append(specSlice, mlxDev+":"+ibDev)
	}
	result.Spec = strings.Join(specSlice, ",")
	var ibDevsSlice []string
	infinibandInfo.RLock()
	for mlxDev, ibDev := range infinibandInfo.IBPFDevs {
		ibDevsSlice = append(ibDevsSlice, mlxDev+":"+ibDev)
	}
	infinibandInfo.RUnlock()
	result.Curr = strings.Join(ibDevsSlice, ",")
	return &result, nil
}
