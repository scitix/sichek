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

func TestCPUUncoreFreqChecker(t *testing.T) {
	pinned := collector.UncoreDomain{Name: "uncore00", MinFreqKHz: 2200000, MaxFreqKHz: 2200000, InitialMaxFreqKHz: 2200000}
	loose := collector.UncoreDomain{Name: "uncore01", MinFreqKHz: 800000, MaxFreqKHz: 2200000, InitialMaxFreqKHz: 2200000}
	tests := []struct {
		name         string
		uncore       collector.UncoreInfo
		wantStatus   string
		wantCurr     string
		detailNeedle string
	}{
		{
			name:       "not applicable",
			uncore:     collector.UncoreInfo{Applicable: false, SkipReason: "virtual machine"},
			wantStatus: consts.StatusNormal,
			wantCurr:   "N/A",
		},
		{
			name:         "driver not loaded",
			uncore:       collector.UncoreInfo{Applicable: true},
			wantStatus:   consts.StatusAbnormal,
			wantCurr:     "DriverNotLoaded",
			detailNeedle: "not loaded",
		},
		{
			name:       "driver without domains",
			uncore:     collector.UncoreInfo{Applicable: true, DriverLoaded: true},
			wantStatus: consts.StatusAbnormal,
			wantCurr:   "NoDomain",
		},
		{
			name: "all pinned",
			uncore: collector.UncoreInfo{Applicable: true, DriverLoaded: true, Interface: collector.UncoreInterfaceTPMI,
				Domains: []collector.UncoreDomain{pinned, pinned}},
			wantStatus: consts.StatusNormal,
			wantCurr:   "tpmi uncore00:2200-2200,uncore00:2200-2200 MHz",
		},
		{
			name: "one domain loose",
			uncore: collector.UncoreInfo{Applicable: true, DriverLoaded: true, Interface: collector.UncoreInterfaceTPMI,
				Domains: []collector.UncoreDomain{pinned, loose}},
			wantStatus:   consts.StatusAbnormal,
			wantCurr:     "tpmi uncore00:2200-2200,uncore01:800-2200 MHz",
			detailNeedle: "1/2 tpmi domain(s): uncore01(min=800,max=2200,ceiling=2200 MHz)",
		},
	}

	checker, err := NewCPUUncoreFreqChecker()
	require.NoError(t, err)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.Check(context.Background(), &collector.CPUOutput{UncoreInfo: tt.uncore})
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, consts.LevelWarning, result.Level)
			assert.Equal(t, tt.wantCurr, result.Curr)
			assert.Contains(t, result.Detail, tt.detailNeedle)
		})
	}
}
