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

package collector

import (
	"context"
	"strings"
	"time"

	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

// DefaultServiceUnit is the systemd unit that performs RDMA NIC initialization. The
// exporter unit (rdma-env-pre-exporter.service) is deliberately not tracked here: the
// scrape of its /metrics already tells us whether it is serving.
const DefaultServiceUnit = "rdma-env-pre.service"

// Derived service states. These collapse systemd's LoadState/ActiveState/SubState/
// Result tuple into the small set an operator actually acts on.
const (
	// ServiceStateNotInstalled means the unit does not exist on this node. It is N/A,
	// not a fault: many clusters do not deploy rdma-env-pre at all.
	ServiceStateNotInstalled = "not_installed"
	// ServiceStateCompleted is a oneshot unit that ran to completion successfully --
	// active (exited). For rdma-env-pre this is the normal steady state after boot.
	ServiceStateCompleted = "completed"
	// ServiceStateRunning is a long-running unit currently up.
	ServiceStateRunning = "running"
	// ServiceStateActivating means it is still starting (rdma-env-pre init takes
	// minutes on a full node); not a fault.
	ServiceStateActivating = "activating"
	// ServiceStateDeactivating means it is shutting down; transient, not a fault.
	ServiceStateDeactivating = "deactivating"
	// ServiceStateFailed means the unit failed, or its last run exited non-zero.
	ServiceStateFailed = "failed"
	// ServiceStateInactive means the unit is installed but stopped and never ran (or
	// was stopped) -- RDMA NIC init has not been applied.
	ServiceStateInactive = "inactive"
	// ServiceStateMasked means the unit is masked and cannot start.
	ServiceStateMasked = "masked"
	// ServiceStateUnknown means systemctl was unavailable or its output unparseable.
	ServiceStateUnknown = "unknown"
)

// showProperties are the systemctl properties queried. Order is irrelevant: systemctl
// prints KEY=VALUE lines that ParseSystemctlShow reads by key.
var showProperties = []string{
	"LoadState", "ActiveState", "SubState", "UnitFileState", "Result",
	"Type", "ExecMainStatus", "NRestarts",
	"ExecMainStartTimestamp", "ExecMainExitTimestamp", "Description",
}

// ServiceStatus is the raw systemd view of one unit, plus the derived State. Fields are
// kept as strings exactly as systemctl reports them so the snapshot shows the ground
// truth an operator would see by hand.
type ServiceStatus struct {
	Unit           string `json:"unit"`
	State          string `json:"state"`
	LoadState      string `json:"load_state"`
	ActiveState    string `json:"active_state"`
	SubState       string `json:"sub_state"`
	UnitFileState  string `json:"unit_file_state,omitempty"`
	Result         string `json:"result,omitempty"`
	Type           string `json:"type,omitempty"`
	ExecMainStatus string `json:"exec_main_status,omitempty"`
	NRestarts      string `json:"n_restarts,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	ExitedAt       string `json:"exited_at,omitempty"`
	Description    string `json:"description,omitempty"`
	// Error records why the query itself failed (systemctl missing, timeout).
	Error string `json:"error,omitempty"`
}

// NotPersistent reports whether the unit will fail to come back after a reboot. Only
// an explicitly disabled unit counts: "static" units have no enable knob, and an empty
// UnitFileState carries no information.
func (s *ServiceStatus) NotPersistent() bool {
	return s.UnitFileState == "disabled"
}

// CollectServiceStatus queries one systemd unit and returns its status with State
// derived. It never returns an error: a failed query yields State=unknown with Error
// set, so a node without systemctl does not fault the whole component.
func CollectServiceStatus(ctx context.Context, unit string, timeout time.Duration) *ServiceStatus {
	if unit == "" {
		unit = DefaultServiceUnit
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]string{"show", unit, "--no-pager"}, propertyArgs()...)
	// `systemctl show` exits 0 even for a unit that does not exist, reporting
	// LoadState=not-found -- so a non-zero exit here means systemctl itself failed.
	out, err := utils.ExecCommand(cctx, "systemctl", args...)
	if err != nil {
		logrus.WithField("component", "rdmaenv").Warnf("systemctl show %s failed: %v", unit, err)
		return &ServiceStatus{Unit: unit, State: ServiceStateUnknown, Error: err.Error()}
	}

	status := ParseSystemctlShow(string(out))
	status.Unit = unit
	status.State = DeriveServiceState(&status)
	return &status
}

func propertyArgs() []string {
	args := make([]string, 0, len(showProperties))
	for _, p := range showProperties {
		args = append(args, "-p", p)
	}
	return args
}

// ParseSystemctlShow parses `systemctl show` KEY=VALUE output. Unknown keys are ignored
// and a line without '=' is skipped; a value may itself contain '=' and is kept whole.
// State is not set here -- call DeriveServiceState.
func ParseSystemctlShow(out string) ServiceStatus {
	var s ServiceStatus
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "LoadState":
			s.LoadState = value
		case "ActiveState":
			s.ActiveState = value
		case "SubState":
			s.SubState = value
		case "UnitFileState":
			s.UnitFileState = value
		case "Result":
			s.Result = value
		case "Type":
			s.Type = value
		case "ExecMainStatus":
			s.ExecMainStatus = value
		case "NRestarts":
			s.NRestarts = value
		case "ExecMainStartTimestamp":
			s.StartedAt = value
		case "ExecMainExitTimestamp":
			s.ExitedAt = value
		case "Description":
			s.Description = value
		}
	}
	return s
}

// DeriveServiceState collapses the systemd tuple into one derived state. Load-level
// verdicts (not-found, masked) win first because they make the active-level fields
// meaningless; then explicit failure; then the active-level state.
func DeriveServiceState(s *ServiceStatus) string {
	switch s.LoadState {
	case "not-found", "bad-setting", "error":
		return ServiceStateNotInstalled
	case "masked":
		return ServiceStateMasked
	}
	if s.UnitFileState == "masked" || s.UnitFileState == "masked-runtime" {
		return ServiceStateMasked
	}

	// Result carries how the last run ended and outlives a restart, so it can report a
	// failure while ActiveState already reads active again.
	if s.ActiveState == "failed" || (s.Result != "" && s.Result != "success") {
		return ServiceStateFailed
	}
	// A oneshot that exited non-zero: systemd may still call it active for units with
	// RemainAfterExit and a tolerant SuccessExitStatus.
	if s.ExecMainStatus != "" && s.ExecMainStatus != "0" {
		return ServiceStateFailed
	}

	switch s.ActiveState {
	case "active":
		if s.SubState == "exited" {
			return ServiceStateCompleted
		}
		return ServiceStateRunning
	case "activating":
		return ServiceStateActivating
	case "deactivating":
		return ServiceStateDeactivating
	case "inactive":
		if s.LoadState == "" {
			return ServiceStateUnknown // no output at all
		}
		return ServiceStateInactive
	}
	return ServiceStateUnknown
}

// ServiceStateIsHealthy reports whether a derived state needs no operator action.
// not_installed is NOT healthy: it is N/A and callers must branch on it separately.
func ServiceStateIsHealthy(state string) bool {
	switch state {
	case ServiceStateCompleted, ServiceStateRunning, ServiceStateActivating, ServiceStateDeactivating:
		return true
	}
	return false
}
