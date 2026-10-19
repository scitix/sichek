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
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

// VBIOSChecker compares each GPU's VBIOS version against spec software.vbios_version.
// An empty spec value means "not enforced" and the check always passes.
type VBIOSChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewVBIOSChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &VBIOSChecker{
		name: config.VBIOSCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *VBIOSChecker) Name() string {
	return c.name
}

func (c *VBIOSChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	nvidiaInfo, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}

	result := config.GPUCheckItems[config.VBIOSCheckerName]
	expected := strings.TrimSpace(c.cfg.Software.VBIOSVersion)
	result.Spec = expected

	versions := make([]string, 0, len(nvidiaInfo.DevicesInfo))
	var mismatched []string
	var mismatchedIdx []string
	for _, device := range nvidiaInfo.DevicesInfo {
		versions = append(versions, fmt.Sprintf("%d:%s", device.Index, device.VBIOSVersion))
		// Unread VBIOS is already reported in PartialErrors; don't turn a collection failure into a mismatch.
		if expected == "" || device.VBIOSVersion == "" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(device.VBIOSVersion), expected) {
			mismatchedIdx = append(mismatchedIdx, fmt.Sprintf("%d", device.Index))
			mismatched = append(mismatched, fmt.Sprintf("%d:%s", device.Index, device.VBIOSVersion))
		}
	}
	result.Curr = strings.Join(versions, ",")

	if len(mismatched) > 0 {
		logrus.WithFields(logrus.Fields{
			"checker":  c.Name(),
			"expected": expected,
			"mismatch": mismatched,
		}).Errorf("VBIOS version mismatch detected")
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("VBIOS version mismatch, expected %s, got %v", expected, mismatched)
		result.Device = strings.Join(mismatchedIdx, ",")
	} else {
		result.Status = consts.StatusNormal
		result.Suggestion = ""
		if expected == "" {
			result.Detail = "VBIOS version not specified in spec, skip check"
		}
	}
	return &result, nil
}
