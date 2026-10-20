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
	"testing"

	"github.com/scitix/sichek/components/rdmaenv/collector"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceChecker(t *testing.T) {
	tests := []struct {
		name       string
		service    *collector.ServiceStatus
		wantStatus string
		wantLevel  string
		wantCurr   string
	}{
		{
			name: "oneshot completed is normal",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateCompleted,
				ActiveState: "active", SubState: "exited", UnitFileState: "enabled",
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
			wantCurr:   collector.ServiceStateCompleted,
		},
		{
			name: "running is normal",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateRunning,
				ActiveState: "active", SubState: "running", UnitFileState: "enabled",
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
			wantCurr:   collector.ServiceStateRunning,
		},
		{
			name: "activating is normal -- init takes minutes and must not alarm at boot",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateActivating,
				ActiveState: "activating", SubState: "start", UnitFileState: "enabled",
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
			wantCurr:   collector.ServiceStateActivating,
		},
		{
			name: "not installed is N/A, not a fault",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateNotInstalled,
				LoadState: "not-found",
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
			wantCurr:   collector.ServiceStateNotInstalled,
		},
		{
			name: "failed is critical",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateFailed,
				ActiveState: "failed", SubState: "failed", Result: "exit-code", ExecMainStatus: "1",
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
			wantCurr:   collector.ServiceStateFailed,
		},
		{
			name: "inactive is warning",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateInactive,
				ActiveState: "inactive", SubState: "dead", UnitFileState: "enabled",
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
			wantCurr:   collector.ServiceStateInactive,
		},
		{
			name: "masked is warning",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateMasked,
				LoadState: "masked", UnitFileState: "masked",
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
			wantCurr:   collector.ServiceStateMasked,
		},
		{
			name: "unknown is warning",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateUnknown,
				Error: "exec: \"systemctl\": executable file not found in $PATH",
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
			wantCurr:   collector.ServiceStateUnknown,
		},
		{
			name: "active but disabled is warning -- will not survive reboot",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre.service", State: collector.ServiceStateCompleted,
				ActiveState: "active", SubState: "exited", UnitFileState: "disabled",
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
			wantCurr:   collector.ServiceStateCompleted,
		},
		{
			name: "static unit is not treated as a persistence problem",
			service: &collector.ServiceStatus{
				Unit: "rdma-env-pre-exporter.service", State: collector.ServiceStateRunning,
				ActiveState: "active", SubState: "running", UnitFileState: "static",
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
			wantCurr:   collector.ServiceStateRunning,
		},
	}

	c, err := NewServiceChecker()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Check(context.Background(), &collector.Info{Service: tt.service})
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantLevel, got.Level)
			assert.Equal(t, tt.wantCurr, got.Curr)
			assert.NotEmpty(t, got.Detail)
		})
	}
}

func TestServiceChecker_NilService(t *testing.T) {
	c, err := NewServiceChecker()
	require.NoError(t, err)

	// Service is never nil in practice, but a nil must not panic or claim health.
	got, err := c.Check(context.Background(), &collector.Info{})
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, got.Status)
	assert.Equal(t, consts.LevelWarning, got.Level)
	assert.Equal(t, collector.ServiceStateUnknown, got.Curr)
}

func TestServiceChecker_WrongDataType(t *testing.T) {
	c, err := NewServiceChecker()
	require.NoError(t, err)

	_, err = c.Check(context.Background(), "not an Info")
	assert.Error(t, err)
}

func TestServiceChecker_ResultIsolation(t *testing.T) {
	// The checker copies the check-item template; a failing check must not leave the
	// shared template mutated for the next call.
	c, err := NewServiceChecker()
	require.NoError(t, err)

	_, err = c.Check(context.Background(), &collector.Info{
		Service: &collector.ServiceStatus{State: collector.ServiceStateFailed},
	})
	require.NoError(t, err)

	got, err := c.Check(context.Background(), &collector.Info{
		Service: &collector.ServiceStatus{State: collector.ServiceStateCompleted, ActiveState: "active", SubState: "exited"},
	})
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, got.Status)
	assert.Equal(t, consts.LevelInfo, got.Level)
	assert.Empty(t, got.Suggestion)
}
