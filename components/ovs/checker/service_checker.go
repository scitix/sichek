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

type ServiceChecker struct{}

func (c *ServiceChecker) Name() string { return config.ServiceCheckerName }

func (c *ServiceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for ServiceChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "OVS daemons active",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	for _, svc := range []string{"openvswitch-switch", "ovs-vswitchd", "ovsdb-server"} {
		if info.Services[svc] != "active" {
			r.Status = consts.StatusAbnormal
			r.Level = consts.LevelCritical
			r.ErrorName = "OVSServiceDown"
			r.Curr = "abnormal"
			r.Detail += fmt.Sprintf("service %s is %q (want active). ", svc, info.Services[svc])
			r.Suggestion = "Check `systemctl status " + svc + "` and DOCA-OVS deployment."
		}
	}
	return r, nil
}
