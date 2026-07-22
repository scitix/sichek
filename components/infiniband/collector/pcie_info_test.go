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
	"github.com/stretchr/testify/require"
)

// fakeBDF writes the four sysfs files for a single PCIe device under root.
// Empty values are skipped so a test can simulate "file missing" cases.
func fakeBDF(t *testing.T, root, bdf, curSpeed, maxSpeed, curWidth, maxWidth string) {
	t.Helper()
	dir := filepath.Join(root, bdf)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	write := func(name, val string) {
		if val == "" {
			return
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(val+"\n"), 0o644))
	}
	write("current_link_speed", curSpeed)
	write("max_link_speed", maxSpeed)
	write("current_link_width", curWidth)
	write("max_link_width", maxWidth)
}

// nicEntryWithSymlink creates the symlink the collector will Readlink. The
// target is a path string that embeds pathBDFs; the collector parses the
// BDF list out of it via regex. The string need not point to a real file.
func nicEntryWithSymlink(t *testing.T, root, entryBDF string, pathBDFs []string) {
	t.Helper()
	parts := []string{"..", "..", "devices", "pci0000:80"}
	parts = append(parts, pathBDFs...)
	target := filepath.Join(parts...)
	require.NoError(t, os.Symlink(target, filepath.Join(root, entryBDF)))
}

func withPCIPath(t *testing.T, root string) {
	t.Helper()
	orig := PCIPath
	PCIPath = root
	t.Cleanup(func() { PCIPath = orig })
}

func TestGetPCIETreeLinksByBDF(t *testing.T) {
	type setup struct {
		// upstreamBDFs are written as real dirs with sysfs files.
		upstream []struct {
			bdf      string
			curSpeed string
			maxSpeed string
			curWidth string
			maxWidth string
		}
		// pathBDFs is the BDF sequence (root → leaf) embedded in the symlink target.
		pathBDFs []string
		// nicBDF is the device passed to getPCIETreeLinksByBDF. The function reads
		// <PCIPath>/<nicBDF> as a symlink (not a directory).
		nicBDF string
	}

	cases := []struct {
		name string
		s    setup
		want []PCIETreeLink
	}{
		{
			name: "full_speed_all_links",
			s: setup{
				upstream: []struct {
					bdf      string
					curSpeed string
					maxSpeed string
					curWidth string
					maxWidth string
				}{
					{"0000:80:01.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
					{"0000:81:00.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
					{"0000:82:00.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
				},
				pathBDFs: []string{"0000:80:01.0", "0000:81:00.0", "0000:82:00.0"},
				nicBDF:   "nic_entry_full",
			},
			want: []PCIETreeLink{
				{
					ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
					CurSpeed: "32.0 GT/s PCIe", CurWidth: "16",
					ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
				{
					ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
					CurSpeed: "32.0 GT/s PCIe", CurWidth: "16",
					ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
			},
		},
		{
			name: "nic_link_downgraded",
			s: setup{
				upstream: []struct {
					bdf      string
					curSpeed string
					maxSpeed string
					curWidth string
					maxWidth string
				}{
					{"0000:80:01.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
					{"0000:81:00.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
					{"0000:82:00.0", "16.0 GT/s PCIe", "32.0 GT/s PCIe", "8", "16"},
				},
				pathBDFs: []string{"0000:80:01.0", "0000:81:00.0", "0000:82:00.0"},
				nicBDF:   "nic_entry_down",
			},
			want: []PCIETreeLink{
				{
					ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
					CurSpeed: "32.0 GT/s PCIe", CurWidth: "16",
					ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
				{
					ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
					CurSpeed: "16.0 GT/s PCIe", CurWidth: "8",
					ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
			},
		},
		{
			name: "root_port_only_supports_gen4",
			s: setup{
				upstream: []struct {
					bdf      string
					curSpeed string
					maxSpeed string
					curWidth string
					maxWidth string
				}{
					{"0000:80:01.0", "16.0 GT/s PCIe", "16.0 GT/s PCIe", "16", "16"},
					{"0000:81:00.0", "16.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
					{"0000:82:00.0", "16.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
				},
				pathBDFs: []string{"0000:80:01.0", "0000:81:00.0", "0000:82:00.0"},
				nicBDF:   "nic_entry_gen4root",
			},
			want: []PCIETreeLink{
				{
					ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
					CurSpeed: "16.0 GT/s PCIe", CurWidth: "16",
					ParentMaxSpeed: "16.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
				{
					ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
					CurSpeed: "16.0 GT/s PCIe", CurWidth: "16",
					ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
			},
		},
		{
			name: "direct_to_cpu_returns_nil",
			s: setup{
				upstream: []struct {
					bdf      string
					curSpeed string
					maxSpeed string
					curWidth string
					maxWidth string
				}{
					{"0000:00:00.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
				},
				pathBDFs: []string{"0000:00:00.0"},
				nicBDF:   "nic_entry_direct",
			},
			want: nil,
		},
		{
			name: "missing_max_speed_on_one_node_keeps_link_with_blank_field",
			s: setup{
				upstream: []struct {
					bdf      string
					curSpeed string
					maxSpeed string
					curWidth string
					maxWidth string
				}{
					{"0000:80:01.0", "32.0 GT/s PCIe", "", "16", "16"},
					{"0000:81:00.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
					{"0000:82:00.0", "32.0 GT/s PCIe", "32.0 GT/s PCIe", "16", "16"},
				},
				pathBDFs: []string{"0000:80:01.0", "0000:81:00.0", "0000:82:00.0"},
				nicBDF:   "nic_entry_missingmax",
			},
			want: []PCIETreeLink{
				{
					ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
					CurSpeed: "32.0 GT/s PCIe", CurWidth: "16",
					ParentMaxSpeed: "", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
				{
					ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
					CurSpeed: "32.0 GT/s PCIe", CurWidth: "16",
					ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
					ParentMaxWidth: "16", ChildMaxWidth: "16",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			withPCIPath(t, root)
			for _, u := range tc.s.upstream {
				fakeBDF(t, root, u.bdf, u.curSpeed, u.maxSpeed, u.curWidth, u.maxWidth)
			}
			nicEntryWithSymlink(t, root, tc.s.nicBDF, tc.s.pathBDFs)

			got := getPCIETreeLinksByBDF(tc.s.nicBDF)
			assert.Equal(t, tc.want, got)
		})
	}
}

// writeFile writes content to dir/name, creating parents.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// symlinkDriver creates <dir>/<bdf>/driver as a symlink to drv. The target
// need not exist as a real path; the collector only reads the symlink's
// basename via os.Readlink.
func symlinkDriver(t *testing.T, dir, bdf, drv string) {
	t.Helper()
	devDir := filepath.Join(dir, bdf)
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(drv, filepath.Join(devDir, "driver")); err != nil {
		t.Fatal(err)
	}
}

func TestGetLostIBPCIeDevices(t *testing.T) {
	root := t.TempDir()
	withPCIPath(t, root)

	// Dead HCA: mellanox, driver mlx5_core, no infiniband/, no net/, no aux -> LOST
	writeFile(t, root, "0000:65:01.0/vendor", "0x15b3\n")
	writeFile(t, root, "0000:65:01.0/device", "0x1021\n")
	symlinkDriver(t, root, "0000:65:01.0", "mlx5_core")

	// Healthy IB: mellanox, driver mlx5_core, has infiniband/ -> not lost
	writeFile(t, root, "0000:67:01.0/vendor", "0x15b3\n")
	symlinkDriver(t, root, "0000:67:01.0", "mlx5_core")
	if err := os.MkdirAll(filepath.Join(root, "0000:67:01.0/infiniband/mlx5_2"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Management NIC: mellanox, driver mlx5_core, has net/, no infiniband/ -> not lost
	writeFile(t, root, "0000:11:00.0/vendor", "0x15b3\n")
	symlinkDriver(t, root, "0000:11:00.0", "mlx5_core")
	if err := os.MkdirAll(filepath.Join(root, "0000:11:00.0/net/eth9"), 0o755); err != nil {
		t.Fatal(err)
	}

	// PCI bridge: mellanox vendor, driver pcieport, no infiniband, no net -> not lost (driver gate)
	writeFile(t, root, "0000:98:00.0/vendor", "0x15b3\n")
	symlinkDriver(t, root, "0000:98:00.0", "pcieport")

	// Aux-RDMA function: mellanox, driver mlx5_core, no infiniband, no net,
	// but has mlx5_core.rdma.* aux device -> not lost (aux gate)
	writeFile(t, root, "0000:aa:00.0/vendor", "0x15b3\n")
	symlinkDriver(t, root, "0000:aa:00.0", "mlx5_core")
	if err := os.MkdirAll(filepath.Join(root, "0000:aa:00.0/mlx5_core.rdma.5"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Non-mellanox, no ib no net -> not lost
	writeFile(t, root, "0000:22:00.0/vendor", "0x8086\n")

	// Virtual function (has physfn), no ib no net -> skipped
	writeFile(t, root, "0000:33:00.1/vendor", "0x15b3\n")
	writeFile(t, root, "0000:33:00.1/physfn", "") // presence => VF

	lost, err := GetLostIBPCIeDevices()
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"0000:65:01.0": "0x15b3:0x1021"}, lost)
}

// TestScanPCIeState_RescansFromDisk reproduces the reboot false-positive:
// the collector must (re)derive its PCIe device sets from the current bus
// state every time it scans, not freeze them at construction. On reboot the
// mlx5 stack binds HCAs asynchronously, so a function can look "lost" (driver
// bound, but infiniband/ and net/ not yet created) at the instant the collector
// is built. A later scan, once the card finishes registering, must clear that.
func TestScanPCIeState_RescansFromDisk(t *testing.T) {
	root := t.TempDir()
	withPCIPath(t, root)

	// A mlx5 HCA mid-registration: vendor + driver bound, but neither
	// infiniband/ nor net/ nor an mlx5_core.rdma.* aux device exists yet.
	writeFile(t, root, "0000:65:01.0/vendor", "0x15b3\n")
	writeFile(t, root, "0000:65:01.0/device", "0x1021\n")
	symlinkDriver(t, root, "0000:65:01.0", "mlx5_core")

	info := &InfinibandInfo{}
	info.scanPCIeState()

	// First scan (construction-time view): the half-registered card looks lost
	// and is not yet RDMA-capable.
	assert.Contains(t, info.IBLostPCIDevs, "0000:65:01.0", "half-registered HCA should scan as lost initially")
	assert.Equal(t, 0, info.IBCapablePCINum, "no infiniband/ yet -> not RDMA-capable")

	// The card finishes registering: its infiniband/ directory appears.
	if err := os.MkdirAll(filepath.Join(root, "0000:65:01.0/infiniband/mlx5_0"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A subsequent scan must reflect the new state, not the frozen one.
	info.scanPCIeState()

	assert.NotContains(t, info.IBLostPCIDevs, "0000:65:01.0", "re-scan must clear a card that finished registering")
	assert.Equal(t, 1, info.IBCapablePCINum, "re-scan must count the now RDMA-capable card")
	assert.Contains(t, info.IBPCIDevs, "0000:65:01.0", "re-scan must refresh IBPCIDevs from disk")
}
