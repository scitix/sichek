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
package lldp

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/lldp/checker"
	"github.com/scitix/sichek/components/lldp/collector"
	"github.com/scitix/sichek/components/lldp/config"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCollector returns a canned snapshot so HealthCheck's checker wiring can
// be exercised without lldpctl or sysfs.
type fakeCollector struct{ info *collector.LldpInfo }

func (f *fakeCollector) Name() string { return "fakeLldpCollector" }
func (f *fakeCollector) Collect(ctx context.Context) (common.Info, error) {
	return f.info, nil
}

// newTestComponent builds a component wired with the given collector and the
// real rail checker, bypassing the NewComponent singleton so each test is
// isolated.
func newTestComponent(t *testing.T, col common.Collector) *component {
	t.Helper()
	railChecker, err := checker.NewRailChecker()
	require.NoError(t, err)
	return &component{
		componentName: consts.ComponentNameLLDP,
		collector:     col,
		checkers:      []common.Checker{railChecker},
		cfg:           &config.LldpUserConfig{LLDP: &config.LldpConfig{}},
		cacheBuffer:   make([]*common.Result, 5),
		cacheInfo:     make([]common.Info, 5),
		cacheSize:     5,
	}
}

func TestHealthCheck_RailInconsistentRollsUpToWarning(t *testing.T) {
	// Two same-model (board_id) cards uplink to different switch port tails —
	// a rail inconsistency the checker must flag through HealthCheck.
	info := &collector.LldpInfo{
		LldpdAvailable: true,
		Interfaces: []collector.IfaceInfo{
			{
				Local: collector.LocalIface{Name: "ib0", BoardID: "boardA"},
				Neighbor: collector.Neighbor{
					Chassis: collector.Chassis{Name: "compute-sw-1"},
					Port:    collector.Port{ID: "FourHundredGigE1/0/7", IDType: "ifname"},
				},
			},
			{
				Local: collector.LocalIface{Name: "ib1", BoardID: "boardA"},
				Neighbor: collector.Neighbor{
					Chassis: collector.Chassis{Name: "compute-sw-2"},
					Port:    collector.Port{ID: "FourHundredGigE1/0/91", IDType: "ifname"},
				},
			},
		},
	}

	c := newTestComponent(t, &fakeCollector{info: info})
	res, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, res.Status)
	assert.Equal(t, consts.LevelWarning, res.Level)
	require.NotEmpty(t, res.Checkers)
}

func TestHealthCheck_RailConsistentIsNormal(t *testing.T) {
	info := &collector.LldpInfo{
		LldpdAvailable: true,
		Interfaces: []collector.IfaceInfo{
			{
				Local: collector.LocalIface{Name: "ib0", BoardID: "boardA"},
				Neighbor: collector.Neighbor{
					Chassis: collector.Chassis{Name: "compute-sw-1"},
					Port:    collector.Port{ID: "FourHundredGigE1/0/7", IDType: "ifname"},
				},
			},
			{
				Local: collector.LocalIface{Name: "ib1", BoardID: "boardA"},
				Neighbor: collector.Neighbor{
					Chassis: collector.Chassis{Name: "compute-sw-2"},
					Port:    collector.Port{ID: "FourHundredGigE1/0/7", IDType: "ifname"},
				},
			},
		},
	}

	c := newTestComponent(t, &fakeCollector{info: info})
	res, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, res.Status)
}

func TestPrintInfo_ReflectsCheckStatus(t *testing.T) {
	c := newTestComponent(t, &fakeCollector{})
	info := &collector.LldpInfo{LldpdAvailable: true}

	t.Run("abnormal result -> PrintInfo returns false (CLI FAIL)", func(t *testing.T) {
		result := &common.Result{
			Status: consts.StatusAbnormal,
			Level:  consts.LevelWarning,
			Checkers: []*common.CheckerResult{
				{Name: "lldp-rail-consistency", Status: consts.StatusAbnormal, Level: consts.LevelWarning},
			},
		}
		assert.False(t, c.PrintInfo(info, result, true))
	})

	t.Run("normal result -> PrintInfo returns true (CLI PASS)", func(t *testing.T) {
		result := &common.Result{
			Status: consts.StatusNormal,
			Level:  consts.LevelInfo,
			Checkers: []*common.CheckerResult{
				{Name: "lldp-rail-consistency", Status: consts.StatusNormal, Level: consts.LevelInfo},
			},
		}
		assert.True(t, c.PrintInfo(info, result, true))
	})
}
