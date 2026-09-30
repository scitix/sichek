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
package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakePort struct {
	name      string
	linkLayer string
	state     string
}

type fakeIBDev struct {
	name  string
	isVF  bool
	ports []fakePort
}

func writeFakeSysfs(t *testing.T, root string, devs []fakeIBDev) {
	t.Helper()
	for _, d := range devs {
		devDir := filepath.Join(root, d.name, "device")
		assert.NoError(t, os.MkdirAll(devDir, 0o755))
		if d.isVF {
			// physfn symlink target does not need to resolve, only its presence is checked.
			assert.NoError(t, os.Symlink("../0000:00:00.0", filepath.Join(devDir, "physfn")))
		}
		for _, p := range d.ports {
			portDir := filepath.Join(root, d.name, "ports", p.name)
			assert.NoError(t, os.MkdirAll(portDir, 0o755))
			assert.NoError(t, os.WriteFile(filepath.Join(portDir, "link_layer"), []byte(p.linkLayer+"\n"), 0o644))
			assert.NoError(t, os.WriteFile(filepath.Join(portDir, "state"), []byte(p.state+"\n"), 0o644))
		}
	}
}

func TestListActiveRoceVFs(t *testing.T) {
	tests := []struct {
		name string
		devs []fakeIBDev
		want []string
	}{
		{
			name: "vf-only host returns sorted active vfs",
			devs: []fakeIBDev{
				{name: "roce_vf_r1", isVF: true, ports: []fakePort{{name: "1", linkLayer: "Ethernet", state: "4: ACTIVE"}}},
				{name: "roce_vf_r0", isVF: true, ports: []fakePort{{name: "1", linkLayer: "Ethernet", state: "4: ACTIVE"}}},
			},
			want: []string{"roce_vf_r0", "roce_vf_r1"},
		},
		{
			name: "multiplane PF and VF mix - only VFs are selected",
			devs: []fakeIBDev{
				{name: "roce_r0", isVF: false, ports: []fakePort{
					{name: "1", linkLayer: "Ethernet", state: "1: DOWN"},
					{name: "2", linkLayer: "Ethernet", state: "4: ACTIVE"},
				}},
				{name: "roce_vf_r0", isVF: true, ports: []fakePort{{name: "1", linkLayer: "Ethernet", state: "4: ACTIVE"}}},
				{name: "roce_vf_r1", isVF: true, ports: []fakePort{{name: "1", linkLayer: "Ethernet", state: "4: ACTIVE"}}},
			},
			want: []string{"roce_vf_r0", "roce_vf_r1"},
		},
		{
			name: "vf with all ports down is excluded",
			devs: []fakeIBDev{
				{name: "roce_vf_dead", isVF: true, ports: []fakePort{
					{name: "1", linkLayer: "Ethernet", state: "1: DOWN"},
					{name: "2", linkLayer: "Ethernet", state: "2: INIT"},
				}},
				{name: "roce_vf_live", isVF: true, ports: []fakePort{{name: "1", linkLayer: "Ethernet", state: "4: ACTIVE"}}},
			},
			want: []string{"roce_vf_live"},
		},
		{
			name: "vf with InfiniBand link layer is excluded",
			devs: []fakeIBDev{
				{name: "ib_vf_0", isVF: true, ports: []fakePort{{name: "1", linkLayer: "InfiniBand", state: "4: ACTIVE"}}},
				{name: "roce_vf_r0", isVF: true, ports: []fakePort{{name: "1", linkLayer: "Ethernet", state: "4: ACTIVE"}}},
			},
			want: []string{"roce_vf_r0"},
		},
		{
			name: "pf-only host returns empty (legacy fallback)",
			devs: []fakeIBDev{
				{name: "mezz_0", isVF: false, ports: []fakePort{{name: "1", linkLayer: "InfiniBand", state: "4: ACTIVE"}}},
				{name: "mlx5_0", isVF: false, ports: []fakePort{{name: "1", linkLayer: "Ethernet", state: "4: ACTIVE"}}},
			},
			want: nil,
		},
		{
			name: "empty sysfs returns nil",
			devs: nil,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFakeSysfs(t, root, tc.devs)
			got := listActiveRoceVFs(root)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("missing sysfs root returns nil without panicking", func(t *testing.T) {
		assert.Nil(t, listActiveRoceVFs(filepath.Join(t.TempDir(), "does-not-exist")))
	})
}
