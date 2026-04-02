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
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

// VendorChecker checks whether the transceiver vendor is in the approved list.
// Skipped when the network spec has check_vendor=false.
type VendorChecker struct {
	spec *config.TransceiverSpec
}

func (c *VendorChecker) Name() string { return config.VendorCheckerName }

func (c *VendorChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for VendorChecker")
	}

	tmpl := config.GetCheckItem(c.Name(), "business")
	result := &common.CheckerResult{
		Name:        tmpl.Name,
		Description: tmpl.Description,
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, module := range info.Modules {
		if !module.Present {
			continue
		}

		netSpec := getNetworkSpec(c.spec, module.NetworkType)
		if netSpec == nil || !netSpec.CheckVendor {
			continue
		}

		vendor := strings.TrimSpace(module.Vendor)
		approved := false
		for _, v := range netSpec.ApprovedVendors {
			if strings.EqualFold(vendor, v) {
				approved = true
				break
			}
		}

		if !approved {
			result.Status = consts.StatusAbnormal
			itemLevel := config.GetCheckItem(c.Name(), module.NetworkType).Level
			if consts.LevelPriority[itemLevel] > consts.LevelPriority[result.Level] {
				result.Level = itemLevel
			}
			result.ErrorName = tmpl.ErrorName
			result.Detail += fmt.Sprintf(
				"Interface %s vendor %q is not in the approved vendors list %v.\n",
				module.Interface, vendor, netSpec.ApprovedVendors,
			)
		}
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = tmpl.Suggestion
	}

	return result, nil
}
