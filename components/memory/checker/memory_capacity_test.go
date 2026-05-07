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

func TestMemoryCapacityChecker(t *testing.T) {
	tests := []struct {
		name         string
		expectedGB   float64
		tolerancePct float64
		capacityGB   float64
		wantStatus   string
		wantLevel    string
	}{
		{
			name:         "matches exactly",
			expectedGB:   512.0,
			tolerancePct: 5.0,
			capacityGB:   512.0,
			wantStatus:   consts.StatusNormal,
			wantLevel:    consts.LevelCritical, // template default
		},
		{
			name:         "within tolerance",
			expectedGB:   512.0,
			tolerancePct: 5.0,
			capacityGB:   500.0, // ~2.3% deviation
			wantStatus:   consts.StatusNormal,
			wantLevel:    consts.LevelCritical,
		},
		{
			name:         "below tolerance",
			expectedGB:   512.0,
			tolerancePct: 5.0,
			capacityGB:   400.0, // ~21.9% deviation
			wantStatus:   consts.StatusAbnormal,
			wantLevel:    consts.LevelCritical,
		},
		{
			name:         "no expected value",
			expectedGB:   0,
			tolerancePct: 5.0,
			capacityGB:   512.0,
			wantStatus:   consts.StatusNormal,
			wantLevel:    consts.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chk, err := NewMemoryCapacityChecker(tt.expectedGB, tt.tolerancePct)
			require.NoError(t, err)

			totalBytes := int64(tt.capacityGB * 1024 * 1024 * 1024)
			data := &collector.Output{
				Capacity: collector.MemoryCapacityInfo{
					TotalBytes: totalBytes,
					TotalGB:    tt.capacityGB,
				},
			}

			result, err := chk.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, MemoryCapacityCheckerName, result.Name)
			assert.Equal(t, tt.wantLevel, result.Level)
		})
	}
}
