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

	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVBIOSChecker(t *testing.T) {
	tests := []struct {
		name        string
		expected    string
		devices     []collector.DeviceInfo
		wantStatus  string
		wantDevices string
		wantCurr    string
	}{
		{
			name:     "empty spec passes but still reports versions",
			expected: "",
			devices: []collector.DeviceInfo{
				{Index: 0, VBIOSVersion: "97.00.88.00.0F"},
				{Index: 1, VBIOSVersion: "97.00.66.00.01"},
			},
			wantStatus: consts.StatusNormal,
			wantCurr:   "0:97.00.88.00.0F,1:97.00.66.00.01",
		},
		{
			name:     "all match, case insensitive",
			expected: "97.00.88.00.0f",
			devices: []collector.DeviceInfo{
				{Index: 0, VBIOSVersion: "97.00.88.00.0F"},
				{Index: 1, VBIOSVersion: "97.00.88.00.0F"},
			},
			wantStatus: consts.StatusNormal,
			wantCurr:   "0:97.00.88.00.0F,1:97.00.88.00.0F",
		},
		{
			name:     "one GPU mismatched",
			expected: "97.00.88.00.0F",
			devices: []collector.DeviceInfo{
				{Index: 0, VBIOSVersion: "97.00.88.00.0F"},
				{Index: 5, VBIOSVersion: "97.00.66.00.01"},
			},
			wantStatus:  consts.StatusAbnormal,
			wantDevices: "5",
			wantCurr:    "0:97.00.88.00.0F,5:97.00.66.00.01",
		},
		{
			name:     "unreadable vbios is not a mismatch",
			expected: "97.00.88.00.0F",
			devices: []collector.DeviceInfo{
				{Index: 0, VBIOSVersion: "97.00.88.00.0F"},
				{Index: 1, VBIOSVersion: ""},
			},
			wantStatus: consts.StatusNormal,
			wantCurr:   "0:97.00.88.00.0F,1:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &config.NvidiaSpec{Software: config.SoftwareSpec{VBIOSVersion: tt.expected}}
			checker, err := NewVBIOSChecker(spec)
			require.NoError(t, err)

			result, err := checker.Check(context.Background(), &collector.NvidiaInfo{DevicesInfo: tt.devices})
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantDevices, result.Device)
			assert.Equal(t, tt.wantCurr, result.Curr)
			assert.Equal(t, consts.LevelWarning, result.Level)
		})
	}
}
