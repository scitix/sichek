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
	"strings"
	"testing"

	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGpuRecoveryActionChecker(t *testing.T) {
	tests := []struct {
		name         string
		devices      []collector.DeviceInfo
		wantStatus   string
		wantDevices  string // comma-joined failed indices, "" when none
		detailNeedle string // substring expected in Detail on abnormal
	}{
		{
			name: "all healthy None",
			devices: []collector.DeviceInfo{
				{Index: 0, UUID: "GPU-a", RecoveryAction: "None"},
				{Index: 1, UUID: "GPU-b", RecoveryAction: "None"},
			},
			wantStatus:  consts.StatusNormal,
			wantDevices: "",
		},
		{
			name: "older driver empty field is healthy",
			devices: []collector.DeviceInfo{
				{Index: 0, UUID: "GPU-a", RecoveryAction: ""},
			},
			wantStatus:  consts.StatusNormal,
			wantDevices: "",
		},
		{
			name: "one GPU needs reset",
			devices: []collector.DeviceInfo{
				{Index: 0, UUID: "GPU-a", RecoveryAction: "None"},
				{Index: 6, UUID: "GPU-bad", RecoveryAction: "Reset"},
			},
			wantStatus:   consts.StatusAbnormal,
			wantDevices:  "6",
			detailNeedle: "GPU-bad(action=Reset)",
		},
		{
			name: "multiple GPUs flagged",
			devices: []collector.DeviceInfo{
				{Index: 3, UUID: "GPU-c", RecoveryAction: "Reboot"},
				{Index: 6, UUID: "GPU-bad", RecoveryAction: "Reset"},
			},
			wantStatus:  consts.StatusAbnormal,
			wantDevices: "3,6",
		},
	}

	checker, err := NewGpuRecoveryActionChecker(&config.NvidiaSpec{})
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &collector.NvidiaInfo{DevicesInfo: tt.devices}
			result, err := checker.Check(context.Background(), info)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantDevices, result.Device)
			if tt.wantStatus == consts.StatusAbnormal {
				// A flagged GPU must be reported at Critical severity (cordon-now
				// semantics), matching the GPUCheckItems template.
				assert.Equal(t, consts.LevelCritical, result.Level)
				if tt.detailNeedle != "" {
					assert.True(t, strings.Contains(result.Detail, tt.detailNeedle),
						"Detail %q should contain %q", result.Detail, tt.detailNeedle)
				}
			}
		})
	}
}

func TestGpuRecoveryActionChecker_WrongType(t *testing.T) {
	checker, err := NewGpuRecoveryActionChecker(&config.NvidiaSpec{})
	require.NoError(t, err)
	_, err = checker.Check(context.Background(), "not-nvidia-info")
	assert.Error(t, err)
}
