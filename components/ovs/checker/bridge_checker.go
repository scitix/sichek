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
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/scitix/sichek/consts"
)

type BridgeChecker struct{ spec *config.OVSSpec }

func (c *BridgeChecker) Name() string { return config.BridgeCheckerName }

func (c *BridgeChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for BridgeChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "Per-rail bridge topology matches Step-10 spec",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	raise := func(level string, errName, detail string) {
		r.Status = consts.StatusAbnormal
		if consts.LevelPriority[r.Level] < consts.LevelPriority[level] {
			r.Level = level
		}
		if r.ErrorName == "" || level == consts.LevelCritical {
			r.ErrorName = errName
		}
		r.Detail += detail
	}

	for _, b := range info.Bridges {
		if !b.Exists {
			raise(consts.LevelCritical, "OVSBridgeMissing", fmt.Sprintf("%s missing. ", b.Name))
			continue
		}
		if b.DatapathType != c.spec.DatapathType {
			raise(consts.LevelCritical, "OVSBridgeDatapath", fmt.Sprintf("%s datapath=%q want %q. ", b.Name, b.DatapathType, c.spec.DatapathType))
		}
		if b.Ports != c.spec.PortsPerBridge {
			raise(consts.LevelCritical, "OVSBridgePorts", fmt.Sprintf("%s ports=%d want %d. ", b.Name, b.Ports, c.spec.PortsPerBridge))
		}
		if b.Flows < c.spec.MinFlows {
			raise(consts.LevelCritical, "OVSBridgeFlows", fmt.Sprintf("%s flows=%d want >=%d. ", b.Name, b.Flows, c.spec.MinFlows))
		}
		if missing := missingInts(c.spec.ExpectedGroupIDs, b.GroupIDs); len(missing) > 0 {
			raise(consts.LevelCritical, "OVSBridgeGroupIDs", fmt.Sprintf("%s missing group_ids=%v. ", b.Name, missing))
		}
		if len(b.OrphanFlowRefs) > 0 {
			raise(consts.LevelCritical, "OVSOrphanFlowRefs", fmt.Sprintf("%s flows reference absent ports=%v. ", b.Name, b.OrphanFlowRefs))
		}
		if len(b.OrphanPorts) > 0 {
			raise(consts.LevelWarning, "OVSOrphanPorts", fmt.Sprintf("%s ports never referenced by a flow=%v. ", b.Name, b.OrphanPorts))
		}
	}
	if r.Status == consts.StatusAbnormal {
		r.Curr = "abnormal"
		r.Suggestion = "Re-run rdma_env_pre Step 10 bridge/group programming for the affected rails."
	}
	return r, nil
}

// missingInts returns elements of want not present in have.
func missingInts(want, have []int) []int {
	hset := make(map[int]bool, len(have))
	for _, v := range have {
		hset[v] = true
	}
	var missing []int
	for _, w := range want {
		if !hset[w] {
			missing = append(missing, w)
		}
	}
	return missing
}
