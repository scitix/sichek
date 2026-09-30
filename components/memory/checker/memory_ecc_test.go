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

	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryECCUncorrectedChecker(t *testing.T) {
	tests := []struct {
		name       string
		edac       collector.EDACInfo
		wantStatus string
		wantLevel  string
	}{
		{
			name: "no UCE",
			edac: collector.EDACInfo{
				Available: true,
				TotalUCE:  0,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelCritical, // template default
		},
		{
			name: "UCE detected",
			edac: collector.EDACInfo{
				Available: true,
				TotalUCE:  3,
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
		},
		{
			name: "EDAC not available",
			edac: collector.EDACInfo{
				Available: false,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chk, err := NewMemoryECCUncorrectedChecker()
			require.NoError(t, err)

			data := &collector.Output{
				EDAC: tt.edac,
			}

			result, err := chk.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, MemoryECCUncorrectedCheckerName, result.Name)
			assert.Equal(t, tt.wantLevel, result.Level)
		})
	}
}

func TestMemoryECCCorrectedChecker(t *testing.T) {
	threshold := int64(100)

	tests := []struct {
		name       string
		edac       collector.EDACInfo
		wantStatus string
		wantLevel  string
	}{
		{
			name: "below threshold",
			edac: collector.EDACInfo{
				Available: true,
				TotalCE:   50,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelWarning, // template default
		},
		{
			name: "at threshold",
			edac: collector.EDACInfo{
				Available: true,
				TotalCE:   100,
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
		},
		{
			name: "above threshold",
			edac: collector.EDACInfo{
				Available: true,
				TotalCE:   150,
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
		},
		{
			name: "not available",
			edac: collector.EDACInfo{
				Available: false,
			},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chk, err := NewMemoryECCCorrectedChecker(threshold)
			require.NoError(t, err)

			data := &collector.Output{
				EDAC: tt.edac,
			}

			result, err := chk.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, MemoryECCCorrectedCheckerName, result.Name)
			assert.Equal(t, tt.wantLevel, result.Level)
		})
	}
}
