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
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDev struct {
	boardID   string
	physState string
}

// writeIBSysfs builds a fake /sys/class/infiniband tree: one dir per device with
// a board_id file and a ports/1/phys_state file.
func writeIBSysfs(t *testing.T, root string, devs map[string]fakeDev) {
	t.Helper()
	for dev, d := range devs {
		portDir := filepath.Join(root, dev, "ports", "1")
		require.NoError(t, os.MkdirAll(portDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, dev, "board_id"), []byte(d.boardID+"\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(portDir, "phys_state"), []byte(d.physState+"\n"), 0o644))
	}
}

func TestMezzNamingResultAt(t *testing.T) {
	const (
		b300Mezz = "NVD0000000079" // B300 mezz board_id
		b200Mezz = "MT_0000001121" // B200 mezz board_id
		cx7Board = "MT_0000000834" // a non-mezz HCA board_id
	)

	tests := []struct {
		name       string
		devs       map[string]fakeDev
		wantStatus string
		wantDevice string
		// substrings expected in Detail
		wantDetail []string
	}{
		{
			name: "mezz named correctly -> normal",
			devs: map[string]fakeDev{
				"mezz_0": {b300Mezz, "5: LinkUp"},
				"mezz_1": {b300Mezz, "2: Polling"},
			},
			wantStatus: consts.StatusNormal,
			wantDetail: []string{"mezz_0 port 1 ==> mezz0 (Up)", "mezz_1 port 1 ==> mezz1 (Down)"},
		},
		{
			name: "mezz misnamed -> critical abnormal",
			devs: map[string]fakeDev{
				"mlx5_9": {b300Mezz, "2: Polling"},
			},
			wantStatus: consts.StatusAbnormal,
			wantDevice: "mlx5_9",
			wantDetail: []string{"mlx5_9 port 1 ==> mlx5_9 (Down)", "expected mezz_<k>"},
		},
		{
			name: "B200 mezz board_id also recognized, named correctly -> normal",
			devs: map[string]fakeDev{
				"mezz_0": {b200Mezz, "5: LinkUp"},
				"mezz_1": {b200Mezz, "5: LinkUp"},
			},
			wantStatus: consts.StatusNormal,
			wantDetail: []string{"mezz_0 port 1 ==> mezz0 (Up)", "mezz_1 port 1 ==> mezz1 (Up)"},
		},
		{
			name: "B200 mezz misnamed -> critical abnormal",
			devs: map[string]fakeDev{
				"mlx5_8": {b200Mezz, "2: Polling"},
			},
			wantStatus: consts.StatusAbnormal,
			wantDevice: "mlx5_8",
			wantDetail: []string{"mlx5_8 port 1 ==> mlx5_8 (Down)", "expected mezz_<k>"},
		},
		{
			name: "no mezz card (only CX7) -> normal",
			devs: map[string]fakeDev{
				"mlx5_0": {cx7Board, "5: LinkUp"},
			},
			wantStatus: consts.StatusNormal,
			wantDetail: []string{"no mezz card found"},
		},
		{
			name: "mixed: one good one bad -> abnormal, only bad reported",
			devs: map[string]fakeDev{
				"mezz_0":  {b300Mezz, "5: LinkUp"},
				"badmezz": {b300Mezz, "5: LinkUp"},
				"mlx5_0":  {cx7Board, "5: LinkUp"},
			},
			wantStatus: consts.StatusAbnormal,
			wantDevice: "badmezz",
			wantDetail: []string{"mezz_0 port 1 ==> mezz0 (Up)", "badmezz port 1 ==> badmezz (Up)  [expected mezz_<k>]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeIBSysfs(t, root, tt.devs)

			got := mezzNamingResultAt(root)

			assert.Equal(t, config.CheckIBMezzName, got.Name)
			assert.Equal(t, consts.LevelCritical, got.Level)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantDevice, got.Device)
			for _, sub := range tt.wantDetail {
				assert.Contains(t, got.Detail, sub)
			}
		})
	}
}

// A host with no IB sysfs at all must pass (non-IB nodes, or IB stack not up).
func TestMezzNamingResultAt_NoSysfs(t *testing.T) {
	got := mezzNamingResultAt(filepath.Join(t.TempDir(), "does-not-exist"))
	assert.Equal(t, consts.StatusNormal, got.Status)
	assert.Equal(t, "no infiniband devices found", got.Detail)
}
