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

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: "Check transceiver link error counter delta",
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

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

		for errType, currVal := range curr {
			prevVal := prev[errType]
			if currVal > prevVal {
				delta := currVal - prevVal
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "LinkErrorsDelta"
				result.Detail += fmt.Sprintf(
					"Interface %s link error %q increased by %d (prev=%d, curr=%d).\n",
					iface, errType, delta, prevVal, currVal,
				)
			}
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
		result.Suggestion = "Inspect cable and transceiver. Persistent link errors may indicate hardware failure."
	}

	return result, nil
}
