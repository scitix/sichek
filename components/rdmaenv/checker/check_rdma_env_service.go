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

// Package checker judges the rdma-env-pre systemd unit's state. It is the only
// judgment the rdmaenv component makes: the scraped /metrics series remain a verbatim
// passthrough.
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/rdmaenv/collector"
	"github.com/scitix/sichek/components/rdmaenv/config"
	"github.com/scitix/sichek/consts"
)

// ServiceChecker verifies that the rdma-env-pre unit has applied its RDMA NIC
// initialization. It is read-only: it never starts, enables, or restarts the unit.
type ServiceChecker struct {
	name string
}

func NewServiceChecker() (common.Checker, error) {
	return &ServiceChecker{name: config.CheckRdmaEnvService}, nil
}

func (c *ServiceChecker) Name() string {
	return c.name
}

func (c *ServiceChecker) GetSpec() common.CheckerSpec {
	return nil
}

// Check maps the derived service state onto a CheckerResult:
//
//	completed / running / activating / deactivating -> normal
//	not_installed                                   -> normal (N/A: node does not use rdma-env-pre)
//	failed                                          -> Critical (NIC init did not succeed)
//	inactive / masked / unknown                     -> Warning
//
// A healthy-but-disabled unit is additionally downgraded to Warning, because its
// settings will not be reapplied after a reboot.
func (c *ServiceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.Info)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.Info")
	}

	result := config.RdmaEnvCheckItems[config.CheckRdmaEnvService]

	svc := info.Service
	if svc == nil {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.Curr = collector.ServiceStateUnknown
		result.Detail = "systemd status for the rdma-env-pre unit was not collected"
		return &result, nil
	}

	result.Device = svc.Unit
	result.Curr = svc.State

	switch svc.State {
	case collector.ServiceStateNotInstalled:
		result.Status = consts.StatusNormal
		result.Level = consts.LevelInfo
		result.Detail = fmt.Sprintf("%s is not installed on this node; RDMA env pre-init check not applicable", svc.Unit)
		result.Suggestion = ""
		return &result, nil

	case collector.ServiceStateFailed:
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.Detail = fmt.Sprintf("%s failed: ActiveState=%s SubState=%s Result=%s ExecMainStatus=%s%s",
			svc.Unit, svc.ActiveState, svc.SubState, svc.Result, svc.ExecMainStatus, exitedSuffix(svc))
		return &result, nil

	case collector.ServiceStateInactive:
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.Detail = fmt.Sprintf("%s is installed but inactive (%s/%s); RDMA NIC settings are not applied",
			svc.Unit, svc.ActiveState, svc.SubState)
		result.Suggestion = "Start the unit (`systemctl start rdma-env-pre`) and confirm it reaches active (exited)"
		return &result, nil

	case collector.ServiceStateMasked:
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.Detail = fmt.Sprintf("%s is masked and cannot start; RDMA NIC settings will never be applied", svc.Unit)
		result.Suggestion = "Unmask the unit (`systemctl unmask rdma-env-pre`) if this node is expected to run RDMA workloads"
		return &result, nil

	case collector.ServiceStateUnknown:
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		detail := fmt.Sprintf("could not determine the state of %s", svc.Unit)
		if svc.Error != "" {
			detail = fmt.Sprintf("%s: %s", detail, svc.Error)
		}
		result.Detail = detail
		result.Suggestion = "Check that systemctl is available and readable from the sichek context"
		return &result, nil
	}

	// Remaining states are healthy: completed, running, activating, deactivating.
	if svc.NotPersistent() {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.Detail = fmt.Sprintf("%s is %s now but its unit file is disabled; RDMA NIC settings will not be reapplied after a reboot",
			svc.Unit, svc.State)
		result.Suggestion = "Enable the unit (`systemctl enable rdma-env-pre`) so initialization survives a reboot"
		return &result, nil
	}

	result.Status = consts.StatusNormal
	result.Level = consts.LevelInfo
	result.Detail = fmt.Sprintf("%s is %s (%s/%s)%s", svc.Unit, svc.State, svc.ActiveState, svc.SubState, exitedSuffix(svc))
	result.Suggestion = ""
	return &result, nil
}

// exitedSuffix appends the last-exit timestamp when systemd reported one, so a failure
// detail says when the run ended rather than only that it did.
func exitedSuffix(svc *collector.ServiceStatus) string {
	if svc.ExitedAt == "" {
		return ""
	}
	return fmt.Sprintf(", last exit at %s", svc.ExitedAt)
}
