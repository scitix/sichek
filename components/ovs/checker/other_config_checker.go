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

type OtherConfigChecker struct{ spec *config.OVSSpec }

func (c *OtherConfigChecker) Name() string { return config.OtherConfigCheckerName }

func (c *OtherConfigChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for OtherConfigChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "Open_vSwitch other_config matches Step-8 spec",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	for k, want := range c.spec.OtherConfig {
		if got := info.OtherConfig[k]; got != want {
			r.Status = consts.StatusAbnormal
			r.Level = consts.LevelCritical
			r.ErrorName = "OVSOtherConfigMismatch"
			r.Detail += fmt.Sprintf("other_config:%s=%q want %q. ", k, got, want)
		}
	}
	if !info.DPDKInitialized {
		r.Status = consts.StatusAbnormal
		r.Level = consts.LevelCritical
		r.ErrorName = "OVSDpdkNotInitialized"
		r.Detail += "dpdk_initialized != true. "
	}
	if r.Status == consts.StatusAbnormal {
		r.Curr = "abnormal"
		r.Suggestion = "Re-apply rdma_env_pre Step 8 other_config and restart ovs-vswitchd."
	}
	return r, nil
}
