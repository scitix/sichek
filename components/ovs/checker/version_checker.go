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

type VersionChecker struct{ spec *config.OVSSpec }

func (c *VersionChecker) Name() string { return config.VersionCheckerName }

func (c *VersionChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for VersionChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "DOCA-OVS packages installed + runtime versions present",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	// Missing package => Critical.
	for _, pkg := range c.spec.RequiredPackages {
		if info.Packages[pkg] == "" {
			r.Status = consts.StatusAbnormal
			r.Level = consts.LevelCritical
			r.ErrorName = "OVSPackageMissing"
			r.Detail += fmt.Sprintf("package %s not installed. ", pkg)
		}
	}
	// Empty runtime version => Warning (only if not already Critical).
	if info.OVSVersion == "" || info.DPDKVersion == "" {
		r.Status = consts.StatusAbnormal
		if consts.LevelPriority[r.Level] < consts.LevelPriority[consts.LevelWarning] {
			r.Level = consts.LevelWarning
		}
		if r.ErrorName == "" {
			r.ErrorName = "OVSRuntimeVersionEmpty"
		}
		r.Detail += "OVS/DPDK runtime version empty (vswitchd may not be connected to DPDK). "
	}
	if r.Status == consts.StatusAbnormal {
		r.Curr = "abnormal"
		r.Suggestion = "Verify DOCA-OVS install (dpkg -l) and that ovs-vswitchd initialized DPDK."
	}
	return r, nil
}
