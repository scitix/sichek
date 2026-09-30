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
	"sort"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type IBLostChecker struct {
	name string
	// spec is retained to satisfy the shared checker-constructor signature;
	// the current checks are spec-independent and do not read it.
	spec        *config.InfinibandSpec
	description string
}

func NewIBLostChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBLostChecker{
		name: config.CheckIBLost,
		spec: specCfg,
	}, nil
}

func (c *IBLostChecker) Name() string {
	return c.name
}

func (c *IBLostChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	defer infinibandInfo.RUnlock()

	lostPCI := infinibandInfo.IBLostPCIDevs

	logrus.WithFields(logrus.Fields{
		"checker":         c.Name(),
		"HCAPCINum":       infinibandInfo.HCAPCINum,
		"IBCapablePCINum": infinibandInfo.IBCapablePCINum,
		"lostPCINum":      len(lostPCI),
	}).Infof("Start IB lost check")

	var details []string
	var devices []string

	// Condition 1: a Mellanox PF present on the PCIe bus with neither
	// infiniband/ nor net/ — a card torn down by a firmware crash or
	// removed from the fabric.
	if len(lostPCI) > 0 {
		result.Status = consts.StatusAbnormal
		bdfs := sortedKeys(lostPCI)
		devices = append(devices, bdfs...)
		details = append(details, fmt.Sprintf("phantom HCA PCIe function(s) with no infiniband/ and no net/: %s", strings.Join(bdfs, ",")))
	}

	// Condition 2 (legacy hardware cross-check): PF count vs RDMA-capable
	// PCIe count. Rarely differs now that the collector aligns them, kept as
	// a safety net.
	if infinibandInfo.HCAPCINum != infinibandInfo.IBCapablePCINum {
		result.Status = consts.StatusAbnormal
		details = append(details, fmt.Sprintf("HCAPCINum != IBCapablePCINum(%d != %d)", infinibandInfo.HCAPCINum, infinibandInfo.IBCapablePCINum))
	}

	if result.Status == consts.StatusAbnormal {
		result.Device = strings.Join(devices, ",")
		result.Detail = "IBLost: " + strings.Join(details, "; ")
		result.Detail += "\nIBCapablePCIDevs: "
		for pciDev := range infinibandInfo.IBPCIDevs {
			result.Detail += pciDev + ","
		}
		result.Detail += "\nIBPFDevs: "
		for ibDev := range infinibandInfo.IBPFDevs {
			result.Detail += ibDev + ","
		}
		logrus.WithFields(logrus.Fields{
			"checker": c.Name(),
			"detail":  result.Detail,
		}).Errorf("IBLost detected")
	}
	return &result, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
