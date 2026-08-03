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
	"fmt"
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// railDev describes one IB device as the collector would report it.
type railDev struct {
	ibDev string
	rate  string // raw sysfs port rate, e.g. "400 Gb/sec (4X NDR)"
	board string // board_id, i.e. the HCA model
	port  int    // 0 means single-port; >0 lets a device appear twice
}

func hwInfoFromRailDevs(devs []railDev) map[string]collector.IBHardWareInfo {
	hws := make(map[string]collector.IBHardWareInfo, len(devs))
	for i, d := range devs {
		port := d.port
		if port == 0 {
			port = 1
		}
		// Key must be unique per (device, port); the real collector uses
		// HWInfoKey for this.
		key := fmt.Sprintf("%s_p%d_%d", d.ibDev, port, i)
		hws[key] = collector.IBHardWareInfo{
			IBDev:     d.ibDev,
			Port:      port,
			PortSpeed: d.rate,
			BoardID:   d.board,
		}
	}
	return hws
}

// rail builds a set of same-model devices sharing one rate.
func rail(board, rate string, names ...string) []railDev {
	devs := make([]railDev, 0, len(names))
	for _, n := range names {
		devs = append(devs, railDev{ibDev: n, rate: rate, board: board})
	}
	return devs
}

// Board IDs and rates as measured on the fleet, so the fixtures stay
// recognisable against real nodes.
const (
	boardCX7    = "MT_0000000838" // 400G compute HCA (bjg66, thg1)
	boardBF3    = "MT_0000001070" // 400G BF3 compute HCA (lmg132, lmg104)
	boardCX6    = "MT_0000000223" // 200G storage HCA (bjg66, dracog24)
	boardDual   = "MT_0000000834" // 200G dual-port HCA (thg1 ib8/ib9, zy3 storage)
	boardMezz   = "MT_0000001121" // 100G mezzanine (thg1)
	boardRoCE   = "NVD0000000072" // 200G RoCE compute HCA (zy3)
	rate400NDR  = "400 Gb/sec (4X NDR)"
	rate200NDR  = "200 Gb/sec (2X NDR)"
	rate200HDR  = "200 Gb/sec (4X HDR)"
	rate100HDR2 = "100 Gb/sec (2X HDR)"
)

// TestIBRailCount_FleetTopologies pins the checker against the real topologies
// measured across the fleet on 2026-07-27. Every healthy node must stay silent
// — the finding is Critical, so a false positive on any of these would cordon
// a working node — and lmg132, which had lost 0000:69:01.0, must be flagged.
func TestIBRailCount_FleetTopologies(t *testing.T) {
	tests := []struct {
		name         string
		devs         []railDev
		wantAbnormal bool
		wantCount    string
	}{
		{
			// longmen-g86-132: BF3 CX7 at 0000:69:01.0 failed its mlx5_core
			// probe, leaving mlx5_1/2/4 where four rails are expected.
			name:         "lmg132 with one lost rail is flagged",
			devs:         rail(boardBF3, rate400NDR, "mlx5_1", "mlx5_2", "mlx5_4"),
			wantAbnormal: true,
			wantCount:    "3",
		},
		{
			name:      "lmg104 four rails",
			devs:      rail(boardBF3, rate400NDR, "mlx5_1", "mlx5_2", "mlx5_3", "mlx5_4"),
			wantCount: "4",
		},
		{
			// lh-g23-141 / lh-g23-299: eight 400G compute rails plus a single
			// 200G storage HCA of a different model. Counting every device would
			// give 9 and warn on a healthy node.
			name: "bjg66 eight compute rails plus one slower storage HCA",
			devs: append(
				rail(boardCX7, rate400NDR, "mlx5_0", "mlx5_1", "mlx5_2", "mlx5_3",
					"mlx5_4", "mlx5_5", "mlx5_6", "mlx5_7"),
				railDev{ibDev: "mlx5_8", rate: rate200HDR, board: boardCX6},
			),
			wantCount: "8",
		},
		{
			// taihua-g92-001: eight 400G rails, two 200G dual-port HCAs and four
			// 100G mezzanine cards — three distinct models.
			name: "thg1 eight compute rails plus two slower HCAs and mezzanines",
			devs: append(append(
				rail(boardCX7, rate400NDR, "mlx5_0", "mlx5_1", "mlx5_2", "mlx5_3",
					"mlx5_4", "mlx5_5", "mlx5_6", "mlx5_7"),
				rail(boardDual, rate200NDR, "mlx5_8", "mlx5_9")...),
				rail(boardMezz, rate100HDR2, "mezz_0", "mezz_1", "mezz_2", "mezz_3")...,
			),
			wantCount: "8",
		},
		{
			// hydra-gpu-214-171-47-3: eight RoCE compute HCAs and two storage
			// HCAs of a different model, all at 200G. Both models tie at the top
			// rate, so both are kept — still even.
			name: "zy3 two models tied at the top rate are both kept",
			devs: append(
				rail(boardRoCE, rate200NDR,
					"roce_r0", "roce_r1", "roce_r2", "roce_r3",
					"roce_r4", "roce_r5", "roce_r6", "roce_r7"),
				rail(boardDual, rate200NDR, "mlx5_0", "mlx5_1")...,
			),
			wantCount: "10",
		},
		{
			// A degraded rail trains at 2X and reports half its nominal rate.
			// Bucketing on the rate figure alone would drop it and turn eight
			// healthy rails into a suspicious seven; its model must keep it in.
			name: "rail degraded to half rate stays counted",
			devs: append(
				rail(boardCX7, rate400NDR, "mlx5_0", "mlx5_1", "mlx5_2",
					"mlx5_4", "mlx5_5", "mlx5_6", "mlx5_7"),
				railDev{ibDev: "mlx5_3", rate: rate200NDR, board: boardCX7},
			),
			wantCount: "8",
		},
		{
			// The same degradation on a node that really has lost a rail must
			// still be flagged, not masked by the model grouping.
			name: "degraded rail plus a genuinely missing one is still odd",
			devs: append(
				rail(boardCX7, rate400NDR, "mlx5_0", "mlx5_1", "mlx5_2",
					"mlx5_4", "mlx5_5", "mlx5_6"),
				railDev{ibDev: "mlx5_3", rate: rate200NDR, board: boardCX7},
			),
			wantAbnormal: true,
			wantCount:    "7",
		},
		{
			// draco-g30-*: one compute plus one storage HCA, same model and rate.
			name:      "dracog24 two same-model HCAs",
			devs:      rail(boardCX6, rate200HDR, "mlx5_0", "mlx5_1"),
			wantCount: "2",
		},
		{
			name:      "single-rail node is legitimate",
			devs:      rail(boardCX7, rate400NDR, "mlx5_0"),
			wantCount: "1",
		},
		{
			// A dual-port HCA yields one IBHardWareInfo entry per port; without
			// per-device dedup this would count 2 and mask an odd rail count.
			name: "dual-port HCA counts once",
			devs: []railDev{
				{ibDev: "mlx5_0", rate: rate400NDR, board: boardCX7, port: 1},
				{ibDev: "mlx5_0", rate: rate400NDR, board: boardCX7, port: 2},
				{ibDev: "mlx5_1", rate: rate400NDR, board: boardCX7, port: 1},
				{ibDev: "mlx5_1", rate: rate400NDR, board: boardCX7, port: 2},
				{ibDev: "mlx5_2", rate: rate400NDR, board: boardCX7, port: 1},
			},
			wantAbnormal: true,
			wantCount:    "3",
		},
		{
			name:      "no devices is not reported here",
			devs:      nil,
			wantCount: "0",
		},
		{
			// An unreadable rate must not be coerced to 0 Gb/sec, which would
			// invent a bogus slowest tier and could flip the verdict.
			name: "device with unparseable rate is excluded",
			devs: append(
				rail(boardCX7, rate400NDR, "mlx5_0", "mlx5_1"),
				railDev{ibDev: "mlx5_2", rate: "", board: boardCX7},
			),
			wantCount: "2",
		},
		{
			// With no board_id there is no model identity, so devices fall back
			// to being grouped by rate — the slower one still drops out.
			name: "missing board_id falls back to rate grouping",
			devs: append(
				rail("", rate400NDR, "mlx5_0", "mlx5_1"),
				railDev{ibDev: "mlx5_2", rate: rate200NDR},
			),
			wantCount: "2",
		},
	}

	checker, err := NewIBRailCountChecker(nil)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &collector.InfinibandInfo{
				IBHardWareInfo: hwInfoFromRailDevs(tt.devs),
			}

			result, err := checker.Check(context.Background(), info)
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, result.Curr, "counted rail devices")

			if tt.wantAbnormal {
				assert.Equal(t, consts.StatusAbnormal, result.Status)
				// Missing compute bandwidth should take the node out of
				// service, so this reports at the same level as IBLost.
				assert.Equal(t, consts.LevelCritical, result.Level)
				// ...but under its own name: unlike IBLost it cannot say
				// which device went away.
				assert.Equal(t, "IBRailCountOdd", result.ErrorName)
			} else {
				assert.Equal(t, consts.StatusNormal, result.Status,
					"healthy topology must not warn: %s", result.Detail)
			}
		})
	}
}
