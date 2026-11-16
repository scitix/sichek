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
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// fakeNodeRoot builds a fake /sys/devices/system/node with n nodeN dirs plus
// the non-node entries the real directory carries.
func fakeNodeRoot(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < n; i++ {
		require.NoError(t, os.Mkdir(filepath.Join(root, "node"+string(rune('0'+i))), 0755))
	}
	for _, f := range []string{"has_cpu", "has_memory", "online", "possible"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte("0-1\n"), 0644))
	}
	require.NoError(t, os.Mkdir(filepath.Join(root, "power"), 0755))
	return root
}

func useNumaFixture(t *testing.T, root, arch string) {
	t.Helper()
	oldRoot, oldArch := numaSysfsRoot, numaCheckArch
	numaSysfsRoot, numaCheckArch = root, arch
	t.Cleanup(func() { numaSysfsRoot, numaCheckArch = oldRoot, oldArch })
}

func gpus(names ...string) *collector.NvidiaInfo {
	info := &collector.NvidiaInfo{}
	for i, n := range names {
		info.DevicesInfo = append(info.DevicesInfo, collector.DeviceInfo{Index: i, Name: n})
	}
	return info
}

func TestExpectedNumaNodes(t *testing.T) {
	tests := []struct {
		name string
		gpu  string
		want int
	}{
		{"B300 SXM6", "NVIDIA B300 SXM6 AC", 4},
		{"B300 lowercase", "nvidia b300", 4},
		{"H200", "NVIDIA H200", 2},
		{"H20", "NVIDIA H20", 2},
		{"B200", "NVIDIA B200", 2},
		{"A100", "NVIDIA A100-PCIE-40GB", 2},
		{"RTX 5090", "NVIDIA GeForce RTX 5090", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, expectedNumaNodes(tt.gpu))
		})
	}
}

func TestCountNumaNodes(t *testing.T) {
	t.Run("four nodes ignoring non-node entries", func(t *testing.T) {
		root := fakeNodeRoot(t, 4)
		require.NoError(t, os.Mkdir(filepath.Join(root, "nodeX"), 0755))
		n, err := countNumaNodes(root)
		require.NoError(t, err)
		assert.Equal(t, 4, n)
	})
	t.Run("missing dir", func(t *testing.T) {
		_, err := countNumaNodes(filepath.Join(t.TempDir(), "nope"))
		assert.Error(t, err)
	})
	t.Run("no node entries", func(t *testing.T) {
		_, err := countNumaNodes(fakeNodeRoot(t, 0))
		assert.Error(t, err)
	})
}

func TestNumaNodesChecker_Check(t *testing.T) {
	tests := []struct {
		name       string
		nodes      int
		arch       string
		override   int
		info       *collector.NvidiaInfo
		wantStatus string
		wantSpec   string
		wantCurr   string
		wantDetail string
	}{
		{name: "B300 with 4 nodes", nodes: 4, arch: "amd64", info: gpus("NVIDIA B300 SXM6 AC"),
			wantStatus: consts.StatusNormal, wantSpec: "4", wantCurr: "4"},
		{name: "H200 with 2 nodes", nodes: 2, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantSpec: "2", wantCurr: "2"},
		{name: "H200 with 4 nodes is drift", nodes: 4, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusAbnormal, wantSpec: "2", wantCurr: "4",
			wantDetail: "NUMA nodes: expected 2 (default for NVIDIA H200), got 4"},
		{name: "spec override accepts 4", nodes: 4, arch: "amd64", override: 4, info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantSpec: "4", wantCurr: "4"},
		{name: "spec override mismatch says override", nodes: 2, arch: "amd64", override: 4, info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusAbnormal, wantSpec: "4", wantCurr: "2",
			wantDetail: "NUMA nodes: expected 4 (spec override), got 2"},
		{name: "negative override ignored", nodes: 4, arch: "amd64", override: -1, info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusAbnormal, wantSpec: "2", wantCurr: "4"},
		{name: "first device name empty uses next", nodes: 4, arch: "amd64", info: gpus("", "NVIDIA B300 SXM6 AC"),
			wantStatus: consts.StatusNormal, wantSpec: "4", wantCurr: "4"},
		{name: "no GPU name skips", nodes: 4, arch: "amd64", info: gpus(),
			wantStatus: consts.StatusNormal},
		{name: "non-amd64 skips", nodes: 9, arch: "arm64", info: gpus("NVIDIA GB300"),
			wantStatus: consts.StatusNormal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useNumaFixture(t, fakeNodeRoot(t, tt.nodes), tt.arch)
			spec := &config.NvidiaSpec{Dependence: config.Dependence{NumaNodes: tt.override}}
			c, err := NewNumaNodesChecker(spec)
			require.NoError(t, err)
			assert.Equal(t, config.NumaNodesCheckerName, c.Name())

			res, err := c.Check(context.Background(), tt.info)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, res.Status)
			assert.Equal(t, consts.LevelWarning, res.Level)
			assert.Equal(t, "NUMANodeCountMismatch", res.ErrorName)
			if tt.wantSpec != "" {
				assert.Equal(t, tt.wantSpec, res.Spec)
				assert.Equal(t, tt.wantCurr, res.Curr)
			}
			if tt.wantDetail != "" {
				assert.Equal(t, tt.wantDetail, res.Detail)
			}
			if tt.wantStatus == consts.StatusAbnormal {
				assert.Contains(t, res.Suggestion, "expects "+tt.wantSpec+" NUMA node(s)")
			}
		})
	}
}

func TestNumaNodesChecker_Errors(t *testing.T) {
	c, err := NewNumaNodesChecker(&config.NvidiaSpec{})
	require.NoError(t, err)

	t.Run("wrong data type", func(t *testing.T) {
		useNumaFixture(t, fakeNodeRoot(t, 2), "amd64")
		_, err := c.Check(context.Background(), "not nvidia info")
		assert.Error(t, err)
	})
	t.Run("sysfs unreadable", func(t *testing.T) {
		useNumaFixture(t, filepath.Join(t.TempDir(), "nope"), "amd64")
		_, err := c.Check(context.Background(), gpus("NVIDIA H200"))
		assert.Error(t, err)
	})
}

func TestDependenceNumaNodesOmitEmpty(t *testing.T) {
	out, err := yaml.Marshal(config.Dependence{PcieAcs: "disable"})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "numa_nodes")

	out, err = yaml.Marshal(config.Dependence{NumaNodes: 4})
	require.NoError(t, err)
	assert.Contains(t, string(out), "numa_nodes: 4")
}
