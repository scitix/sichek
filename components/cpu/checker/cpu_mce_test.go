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

func TestCPUMCEUncorrectedChecker(t *testing.T) {
	tests := []struct {
		name       string
		mceInfo    collector.MCEInfo
		wantStatus string
		wantLevel  string
	}{
		{
			name: "no UCE",
			mceInfo: collector.MCEInfo{
				Available:        true,
				UncorrectedCount: 0,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelCritical, // template default
		},
		{
			name: "UCE detected",
			mceInfo: collector.MCEInfo{
				Available:        true,
				UncorrectedCount: 3,
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
		},
		{
			name: "MCE not available",
			mceInfo: collector.MCEInfo{
				Available: false,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewCPUMCEUncorrectedChecker()
			require.NoError(t, err)

			data := &collector.CPUOutput{
				MCEInfo: tt.mceInfo,
			}

			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, CPUMCEUncorrectedCheckerName, result.Name)
			assert.Equal(t, tt.wantLevel, result.Level)
		})
	}
}

func TestCPUMCECorrectedChecker(t *testing.T) {
	threshold := int64(10)

	tests := []struct {
		name       string
		mceInfo    collector.MCEInfo
		wantStatus string
		wantLevel  string
	}{
		{
			name: "below threshold",
			mceInfo: collector.MCEInfo{
				Available:      true,
				CorrectedCount: 5,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelWarning, // template default
		},
		{
			name: "above threshold",
			mceInfo: collector.MCEInfo{
				Available:      true,
				CorrectedCount: 15,
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
		},
		{
			name: "not available",
			mceInfo: collector.MCEInfo{
				Available: false,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewCPUMCECorrectedChecker(threshold)
			require.NoError(t, err)

			data := &collector.CPUOutput{
				MCEInfo: tt.mceInfo,
			}

			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, CPUMCECorrectedCheckerName, result.Name)
			assert.Equal(t, tt.wantLevel, result.Level)
		})
	}
}
