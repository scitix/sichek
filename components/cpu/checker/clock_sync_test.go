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

	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockSyncServiceChecker(t *testing.T) {
	tests := []struct {
		name       string
		ptpInfo    collector.PTPInfo
		wantStatus string
	}{
		{
			name: "PTP active",
			ptpInfo: collector.PTPInfo{
				PTPServiceActive: true,
				SyncAvailable:    true,
			},
			wantStatus: consts.StatusNormal,
		},
		{
			name: "NTP active",
			ptpInfo: collector.PTPInfo{
				NTPServiceActive: true,
				SyncAvailable:    true,
			},
			wantStatus: consts.StatusNormal,
		},
		{
			name: "no sync service",
			ptpInfo: collector.PTPInfo{
				SyncAvailable: false,
			},
			wantStatus: consts.StatusAbnormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewClockSyncServiceChecker()
			require.NoError(t, err)

			data := &collector.CPUOutput{
				PTPInfo: tt.ptpInfo,
			}

			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, ClockSyncServiceCheckerName, result.Name)
		})
	}
}

func TestClockSyncOffsetChecker(t *testing.T) {
	warningMs := 1.0
	criticalMs := 10.0

	tests := []struct {
		name       string
		ptpInfo    collector.PTPInfo
		wantStatus string
		wantLevel  string
	}{
		{
			name: "within threshold",
			ptpInfo: collector.PTPInfo{
				PTPServiceActive: true,
				SyncAvailable:    true,
				OffsetNs:         500000, // 0.5ms
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelWarning, // template default, but Normal so doesn't matter
		},
		{
			name: "exceeds warning",
			ptpInfo: collector.PTPInfo{
				PTPServiceActive: true,
				SyncAvailable:    true,
				OffsetNs:         2000000, // 2ms
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
		},
		{
			name: "exceeds critical",
			ptpInfo: collector.PTPInfo{
				PTPServiceActive: true,
				SyncAvailable:    true,
				OffsetNs:         15000000, // 15ms
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
		},
		{
			name: "no sync available",
			ptpInfo: collector.PTPInfo{
				SyncAvailable: false,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelWarning, // template default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewClockSyncOffsetChecker(warningMs, criticalMs)
			require.NoError(t, err)

			data := &collector.CPUOutput{
				PTPInfo: tt.ptpInfo,
			}

			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, ClockSyncOffsetCheckerName, result.Name)

			if tt.wantStatus == consts.StatusAbnormal {
				assert.Equal(t, tt.wantLevel, result.Level)
			}
		})
	}
}
