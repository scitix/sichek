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

// fakeBDFNetdev creates /sys/bus/pci/devices/<bdf>/net/<iface> under root so
// shouldSkipSecondaryFunction can probe whether a PCI function exposes a netdev.
// Pass iface == "" to create the net/ dir but leave it empty.
func fakeBDFNetdev(t *testing.T, root, bdf, iface string) {
	t.Helper()
	netDir := filepath.Join(root, bdf, "net")
	require.NoError(t, os.MkdirAll(netDir, 0o755))
	if iface != "" {
		require.NoError(t, os.MkdirAll(filepath.Join(netDir, iface), 0o755))
	}
}

// shouldSkipSecondaryFunction must NOT drop the independent second port of a
// dual-port HCA. A ".1" PCI function that exposes its own netdev is a genuine
// IB/RoCE port and must be enumerated even when its netdev is not enslaved to a
// bond — the previous "keep only if bonded" rule silently dropped healthy
// non-bonded second ports (field-confirmed: thg1 mlx5_9/ib9, zy3 mlx5_1/s_eth1).
func TestShouldSkipSecondaryFunction(t *testing.T) {
	tests := []struct {
		name string
		bdf  string
		// netdev under the bdf: "<iface>" present, "" empty net/ dir, "-" no net/ dir
		netdev string
		want   bool
	}{
		{
			name:   "primary function .0 is never skipped",
			bdf:    "0000:a9:00.0",
			netdev: "-", // .0 short-circuits before any net/ probe
			want:   false,
		},
		{
			name:   "independent second port .1 with netdev is kept (the fix)",
			bdf:    "0000:a9:00.1",
			netdev: "ib9",
			want:   false,
		},
		{
			name:   "RoCE second port .1 with netdev is kept",
			bdf:    "0000:e2:00.1",
			netdev: "s_eth1",
			want:   false,
		},
		{
			name:   "second function .1 with empty net dir is skipped (phantom)",
			bdf:    "0000:a9:00.1",
			netdev: "",
			want:   true,
		},
		{
			name:   "second function .1 with no net dir is skipped",
			bdf:    "0000:a9:00.1",
			netdev: "-",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			withPCIPath(t, root)
			switch tt.netdev {
			case "-":
				// no net dir at all
			default:
				fakeBDFNetdev(t, root, tt.bdf, tt.netdev)
			}
			got := shouldSkipSecondaryFunction(tt.bdf)
			assert.Equal(t, tt.want, got)
		})
	}
}
