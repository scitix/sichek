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
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

// LinkErrorsChecker checks for increasing link error counters between consecutive
// calls. Skipped when the network spec has check_link_errors=false.
type LinkErrorsChecker struct {
	spec       *config.TransceiverSpec
	mu         sync.Mutex
	prevErrors map[string]map[string]uint64
}

func (c *LinkErrorsChecker) Name() string { return config.LinkErrorsCheckerName }

func (c *LinkErrorsChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for LinkErrorsChecker")
	}

	tmpl := config.GetCheckItem(c.Name(), "business")
	result := &common.CheckerResult{
		Name:        tmpl.Name,
		Description: tmpl.Description,
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	var abnormalDevices []string

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, module := range info.Modules {
		if !module.Present {
			continue
		}

		netSpec := getNetworkSpec(c.spec, module.NetworkType)
		if netSpec == nil || !netSpec.CheckLinkErrors {
			continue
		}

		iface := module.Interface
		curr := module.LinkErrors

		prev, hasPrev := c.prevErrors[iface]
		if !hasPrev {
			// First observation — store and move on
			snapshot := make(map[string]uint64, len(curr))
			for k, v := range curr {
				snapshot[k] = v
			}
			c.prevErrors[iface] = snapshot
			continue
		}

		moduleAbnormal := false
		for errType, currVal := range curr {
			prevVal := prev[errType]
			if currVal > prevVal {
				delta := currVal - prevVal
				result.Status = consts.StatusAbnormal
				itemLevel := config.GetCheckItem(c.Name(), module.NetworkType).Level
				if consts.LevelPriority[itemLevel] > consts.LevelPriority[result.Level] {
					result.Level = itemLevel
				}
				result.ErrorName = tmpl.ErrorName
				result.Detail += fmt.Sprintf(
					"Interface %s link error %q increased by %d (prev=%d, curr=%d).\n",
					iface, errType, delta, prevVal, currVal,
				)
				moduleAbnormal = true
			}
		}
		if moduleAbnormal {
			abnormalDevices = append(abnormalDevices, iface)
		}

		// Update snapshot
		snapshot := make(map[string]uint64, len(curr))
		for k, v := range curr {
			snapshot[k] = v
		}
		c.prevErrors[iface] = snapshot
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = tmpl.Suggestion
	}
	if len(abnormalDevices) > 0 {
		result.Device = strings.Join(abnormalDevices, ",")
	}

	return result, nil
}
