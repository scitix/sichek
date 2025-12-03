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
	"os"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
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

	var mismatchPairs []string
	infinibandInfo.RLock()
	for expectedMlx5 := range c.spec.IBPFDevs {
		actualIb, found := infinibandInfo.IBPFDevs[expectedMlx5]
		if !found {
			mismatchPairs = append(mismatchPairs, fmt.Sprintf("%s (missing)", expectedMlx5))
			continue
		}

		var expectedIb string
		parts := strings.Split(expectedMlx5, "_")

		if len(parts) > 1 {
			numberStr := parts[len(parts)-1]
			switch infinibandInfo.IBHardWareInfo[expectedMlx5].LinkLayer {
			case "Ethernet":
				// Just for js cluster which uses eth1X for roce device naming
				name, err := os.Hostname()
				if err != nil {
					logrus.WithField("component", "infiniband").Errorf("fail to get hostname: %v", err)
					os.Exit(1)
				}
				if strings.HasPrefix(name, "js") {
					expectedIb = "eth1" + numberStr
					break
				}
				expectedIb = "eth" + numberStr
			case "InfiniBand":
				expectedIb = "ib" + numberStr
			}

		} else {
			logrus.WithField("component", "infiniband").Warnf("fail to extract number from '%s'.", expectedMlx5)
		}

		if actualIb != expectedIb {
			mismatchPairs = append(mismatchPairs, fmt.Sprintf("%s -> %s (expected %s)", expectedMlx5, actualIb, expectedIb))
		}
	}
	infinibandInfo.RUnlock()

	if len(mismatchPairs) > 0 {
		result.Status = consts.StatusAbnormal
		result.Device = strings.Join(mismatchPairs, ",")
		// Read IBPFDevs under lock protection
		infinibandInfo.RLock()
		ibpfDevsCopy := make(map[string]string)
		for k, v := range infinibandInfo.IBPFDevs {
			ibpfDevsCopy[k] = v
		}
		infinibandInfo.RUnlock()
		result.Detail = fmt.Sprintf("Mismatched IB devices %v, expected: %v", ibpfDevsCopy, c.spec.IBPFDevs)
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
