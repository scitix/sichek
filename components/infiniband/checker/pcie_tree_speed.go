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
	"math"
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

// pcieSpeedEqual compares two PCIe speed strings ("32", "32.0", "32.00 GT/s")
// numerically so spec authors do not have to mirror sysfs's trailing-zero
// formatting verbatim.  Falls back to string equality when either value is
// not parseable, preserving prior behaviour for free-form spec values.
func pcieSpeedEqual(a, b string) bool {
	af, errA := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(a)), 64)
	bf, errB := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(b)), 64)
	if errA != nil || errB != nil {
		return a == b
	}
	return math.Abs(af-bf) < 1e-9
}

type IBPCIETreeSpeedChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBPCIETreeSpeedChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBPCIETreeSpeedChecker{
		id:   consts.CheckerIDInfinibandFW,
		name: config.CheckPCIETreeSpeed,
		spec: specCfg,
	}, nil
}

func (c *IBPCIETreeSpeedChecker) Name() string {
	return c.name
}

func (c *IBPCIETreeSpeedChecker) Description() string {
	return c.description
}

func (c *IBPCIETreeSpeedChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBPCIETreeSpeedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	hwInfoLen := len(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	if hwInfoLen == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	failedDevices := make([]string, 0)
	spec := make([]string, 0, hwInfoLen)
	curr := make([]string, 0, hwInfoLen)
	var failedSpec []string
	var failedCurr []string

	infinibandInfo.RLock()
	hws := uniqueByDev(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()
	for _, hwInfo := range hws {
		if _, ok := c.spec.HCAs[hwInfo.BoardID]; !ok {
			logrus.WithField("component", "infiniband").Warnf("HCA %s not found in spec, skipping %s", hwInfo.BoardID, c.name)
			continue
		}
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		// Tree-speed has its own spec field (yaml: pcie_tree_speed) because
		// upstream switches and root complexes are often slower than the
		// device-level link.  CX8 e.g. links at PCIe Gen6 (64 GT/s) but the
		// upstream Gen5 switch caps the path at 32 GT/s.  Fall back to
		// PCIESpeed for board specs that predate the dedicated field.
		treeSpec := hcaSpec.Hardware.PCIETreeSpeedMin
		if treeSpec == "" {
			treeSpec = hcaSpec.Hardware.PCIESpeed
		}
		expectedSpeed := extractNumericSpeed(treeSpec)
		spec = append(spec, treeSpec)

		treeSpeedMin := hwInfo.PCIETreeSpeedMin
		if treeSpeedMin == "" {
			// No upstream tree info available (e.g., direct to CPU), skip
			curr = append(curr, hwInfo.PCIESpeed)
			continue
		}
		curr = append(curr, treeSpeedMin)

		// Compare numerically so "32" / "32.0" / "32.00" all match without
		// requiring spec authors to keep the trailing zero in sync with sysfs.
		if !pcieSpeedEqual(treeSpeedMin, expectedSpeed) {
			result.Status = consts.StatusAbnormal
			devInfo := fmt.Sprintf("%s(%s)", hwInfo.IBDev, hwInfo.PCIEBDF)
			if hwInfo.PCIETreeSpeedMinBDF != "" {
				devInfo = fmt.Sprintf("%s(%s, bottleneck@%s)", hwInfo.IBDev, hwInfo.PCIEBDF, hwInfo.PCIETreeSpeedMinBDF)
			}
			failedDevices = append(failedDevices, devInfo)
			failedSpec = append(failedSpec, treeSpec)
			failedCurr = append(failedCurr, treeSpeedMin)
		}
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedDevices, ",")
	if len(failedDevices) != 0 {
		result.Detail = fmt.Sprintf("PCIETreeSpeed check fail: %s upstream path min speed %s, expect %s", strings.Join(failedDevices, ","), strings.Join(failedCurr, ","), strings.Join(failedSpec, ","))
		result.Suggestion = fmt.Sprintf("Check upstream PCIe switch/bridge speed for %s, expected %s but found %s in path to root complex", strings.Join(failedDevices, ","), strings.Join(failedSpec, ","), strings.Join(failedCurr, ","))
	}

	return &result, nil
}

// extractNumericSpeed extracts the numeric part from a PCIe speed string.
// e.g., "32.0 GT/s PCIe" -> "32.0", "16.0" -> "16.0"
func extractNumericSpeed(speed string) string {
	parts := strings.Fields(speed)
	if len(parts) == 0 {
		return speed
	}
	return parts[0]
}
