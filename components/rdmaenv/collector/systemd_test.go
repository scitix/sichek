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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

func TestParseSystemctlShow(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    ServiceStatus
	}{
		{
			name:    "oneshot finished successfully",
			fixture: "systemctl_show_completed.txt",
			want: ServiceStatus{
				LoadState:      "loaded",
				ActiveState:    "active",
				SubState:       "exited",
				UnitFileState:  "enabled",
				Result:         "success",
				Type:           "oneshot",
				ExecMainStatus: "0",
				NRestarts:      "0",
				StartedAt:      "Sun 2026-09-27 13:50:08 UTC",
				ExitedAt:       "Sun 2026-09-27 13:51:50 UTC",
			},
		},
		{
			name:    "unit not installed",
			fixture: "systemctl_show_notfound.txt",
			want: ServiceStatus{
				LoadState:   "not-found",
				ActiveState: "inactive",
				SubState:    "dead",
				Result:      "success",
				Type:        "oneshot",
				NRestarts:   "0",
			},
		},
		{
			name:    "unit failed with non-zero exit",
			fixture: "systemctl_show_failed.txt",
			want: ServiceStatus{
				LoadState:      "loaded",
				ActiveState:    "failed",
				SubState:       "failed",
				UnitFileState:  "enabled",
				Result:         "exit-code",
				Type:           "oneshot",
				ExecMainStatus: "1",
				NRestarts:      "0",
				StartedAt:      "Sun 2026-09-27 13:50:08 UTC",
				ExitedAt:       "Sun 2026-09-27 13:50:11 UTC",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSystemctlShow(readFixture(t, tt.fixture))
			assert.Equal(t, tt.want.LoadState, got.LoadState)
			assert.Equal(t, tt.want.ActiveState, got.ActiveState)
			assert.Equal(t, tt.want.SubState, got.SubState)
			assert.Equal(t, tt.want.UnitFileState, got.UnitFileState)
			assert.Equal(t, tt.want.Result, got.Result)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.ExecMainStatus, got.ExecMainStatus)
			assert.Equal(t, tt.want.NRestarts, got.NRestarts)
			assert.Equal(t, tt.want.StartedAt, got.StartedAt)
			assert.Equal(t, tt.want.ExitedAt, got.ExitedAt)
		})
	}
}

func TestParseSystemctlShow_MalformedLines(t *testing.T) {
	// A blank line, a line with no '=', and a value that itself contains '=' must not
	// derail the parse: systemctl emits Environment=FOO=bar style values.
	got := ParseSystemctlShow("\nLoadState=loaded\ngarbage-no-equals\nDescription=a=b\nActiveState=active\n")
	assert.Equal(t, "loaded", got.LoadState)
	assert.Equal(t, "active", got.ActiveState)
	assert.Equal(t, "a=b", got.Description)
}

func TestDeriveServiceState(t *testing.T) {
	tests := []struct {
		name   string
		status ServiceStatus
		want   string
	}{
		{
			name:   "not-found is not installed",
			status: ServiceStatus{LoadState: "not-found", ActiveState: "inactive", SubState: "dead"},
			want:   ServiceStateNotInstalled,
		},
		{
			name:   "masked",
			status: ServiceStatus{LoadState: "masked", ActiveState: "inactive", SubState: "dead", UnitFileState: "masked"},
			want:   ServiceStateMasked,
		},
		{
			name:   "oneshot finished is completed",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "active", SubState: "exited", Result: "success", ExecMainStatus: "0"},
			want:   ServiceStateCompleted,
		},
		{
			name:   "long-running is running",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "active", SubState: "running", Result: "success", ExecMainStatus: "0"},
			want:   ServiceStateRunning,
		},
		{
			name:   "still starting up is activating",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "activating", SubState: "start", Result: "success"},
			want:   ServiceStateActivating,
		},
		{
			name:   "shutting down is deactivating",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "deactivating", SubState: "stop", Result: "success"},
			want:   ServiceStateDeactivating,
		},
		{
			name:   "ActiveState failed",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "failed", SubState: "failed", Result: "exit-code", ExecMainStatus: "1"},
			want:   ServiceStateFailed,
		},
		{
			name: "active but Result records a past failure",
			// A restarted unit can be active again while Result still reports how the
			// previous run ended; that is a real fault worth surfacing.
			status: ServiceStatus{LoadState: "loaded", ActiveState: "active", SubState: "running", Result: "timeout"},
			want:   ServiceStateFailed,
		},
		{
			name:   "exited with non-zero status",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "active", SubState: "exited", Result: "success", ExecMainStatus: "2"},
			want:   ServiceStateFailed,
		},
		{
			name:   "loaded but stopped",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "inactive", SubState: "dead", Result: "success"},
			want:   ServiceStateInactive,
		},
		{
			name:   "empty output is unknown",
			status: ServiceStatus{},
			want:   ServiceStateUnknown,
		},
		{
			name:   "unrecognised ActiveState is unknown",
			status: ServiceStatus{LoadState: "loaded", ActiveState: "reloading", Result: "success"},
			want:   ServiceStateUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DeriveServiceState(&tt.status))
		})
	}
}

func TestServiceStateHelpers(t *testing.T) {
	assert.True(t, ServiceStateIsHealthy(ServiceStateCompleted))
	assert.True(t, ServiceStateIsHealthy(ServiceStateRunning))
	assert.True(t, ServiceStateIsHealthy(ServiceStateActivating))
	assert.True(t, ServiceStateIsHealthy(ServiceStateDeactivating))
	assert.False(t, ServiceStateIsHealthy(ServiceStateFailed))
	assert.False(t, ServiceStateIsHealthy(ServiceStateInactive))
	assert.False(t, ServiceStateIsHealthy(ServiceStateMasked))
	assert.False(t, ServiceStateIsHealthy(ServiceStateUnknown))
	// not_installed is N/A, not healthy: callers must branch on it before judging.
	assert.False(t, ServiceStateIsHealthy(ServiceStateNotInstalled))
}

func TestParseSystemctlShow_PersistenceConcern(t *testing.T) {
	// A unit that runs now but is disabled will not come back after a reboot.
	st := ParseSystemctlShow("LoadState=loaded\nActiveState=active\nSubState=exited\nResult=success\nExecMainStatus=0\nUnitFileState=disabled\n")
	assert.Equal(t, ServiceStateCompleted, DeriveServiceState(&st))
	assert.True(t, st.NotPersistent())

	// "static" units have no enable/disable knob at all -- not a concern.
	st = ParseSystemctlShow("LoadState=loaded\nActiveState=active\nSubState=running\nResult=success\nExecMainStatus=0\nUnitFileState=static\n")
	assert.False(t, st.NotPersistent())

	st = ParseSystemctlShow("LoadState=loaded\nActiveState=active\nSubState=exited\nResult=success\nExecMainStatus=0\nUnitFileState=enabled\n")
	assert.False(t, st.NotPersistent())
}
