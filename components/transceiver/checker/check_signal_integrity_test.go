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

	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalIntegrityChecker(t *testing.T) {
	tests := []struct {
		name       string
		modules    []collector.ModuleInfo
		wantStatus string
		wantLevel  string
		wantDevice string // substring expected in result.Device ("" = don't check)
	}{
		{
			name: "business bad signal integrity is Critical even with zero counters",
			modules: []collector.ModuleInfo{
				{Interface: "eth0", NetworkType: "business", Present: true,
					Recommendation: "Bad signal integrity",
					LinkErrors:     map[string]uint64{"symbol_error_counter": 0}},
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
			wantDevice: "eth0",
		},
		{
			name: "management bad signal integrity is Warning",
			modules: []collector.ModuleInfo{
				{Interface: "mgmt0", NetworkType: "management", Present: true,
					Recommendation: "Bad signal integrity"},
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
			wantDevice: "mgmt0",
		},
		{
			name: "case-insensitive match",
			modules: []collector.ModuleInfo{
				{Interface: "eth0", NetworkType: "business", Present: true,
					Recommendation: "BAD SIGNAL INTEGRITY"},
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
		},
		{
			name: "healthy recommendation is Normal",
			modules: []collector.ModuleInfo{
				{Interface: "eth0", NetworkType: "business", Present: true,
					Recommendation: "No issue was observed"},
			},
			wantStatus: consts.StatusNormal,
		},
		{
			name: "empty recommendation is Normal",
			modules: []collector.ModuleInfo{
				{Interface: "eth0", NetworkType: "business", Present: true, Recommendation: ""},
			},
			wantStatus: consts.StatusNormal,
		},
		{
			name: "absent module is skipped even with bad recommendation",
			modules: []collector.ModuleInfo{
				{Interface: "eth0", NetworkType: "business", Present: false,
					Recommendation: "Bad signal integrity"},
			},
			wantStatus: consts.StatusNormal,
		},
		{
			name: "highest level wins across modules",
			modules: []collector.ModuleInfo{
				{Interface: "mgmt0", NetworkType: "management", Present: true,
					Recommendation: "Bad signal integrity"},
				{Interface: "eth0", NetworkType: "business", Present: true,
					Recommendation: "Bad signal integrity"},
			},
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
		},
	}

	chk := &SignalIntegrityChecker{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &collector.TransceiverInfo{Modules: tt.modules}
			result, err := chk.Check(context.Background(), info)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			if tt.wantLevel != "" {
				assert.Equal(t, tt.wantLevel, result.Level)
			}
			if tt.wantStatus == consts.StatusAbnormal {
				assert.Equal(t, "BadSignalIntegrity", result.ErrorName)
				assert.NotEmpty(t, result.Suggestion)
			}
			if tt.wantDevice != "" {
				assert.True(t, strings.Contains(result.Device, tt.wantDevice),
					"expected device %q to contain %q", result.Device, tt.wantDevice)
			}
		})
	}
}

func TestSignalIntegrityChecker_InvalidDataType(t *testing.T) {
	chk := &SignalIntegrityChecker{}
	_, err := chk.Check(context.Background(), "not a TransceiverInfo")
	assert.Error(t, err)
}

func TestNewCheckers_IncludesSignalIntegrity(t *testing.T) {
	checkers, err := NewCheckers(nil, testSpec())
	require.NoError(t, err)

	found := false
	for _, chk := range checkers {
		if chk.Name() == config.SignalIntegrityCheckerName {
			found = true
			break
		}
	}
	assert.True(t, found, "NewCheckers should include the signal integrity checker")
}
