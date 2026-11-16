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
	"strconv"
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
		require.NoError(t, os.Mkdir(filepath.Join(root, "node"+strconv.Itoa(i)), 0755))
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
		spec       int
		nodes      int
		arch       string
		info       *collector.NvidiaInfo
		wantStatus string
		wantSpec   string
		wantCurr   string
		wantDetail string
	}{
		{name: "spec 2, nodes 2", spec: 2, nodes: 2, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantSpec: "2", wantCurr: "2", wantDetail: "NUMA nodes: 2"},
		{name: "spec 4, nodes 4", spec: 4, nodes: 4, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantSpec: "4", wantCurr: "4"},
		{name: "spec 2, nodes 4", spec: 2, nodes: 4, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusAbnormal, wantSpec: "2", wantCurr: "4", wantDetail: "NUMA nodes: expected 2, got 4"},
		{name: "spec 4, nodes 2", spec: 4, nodes: 2, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusAbnormal, wantSpec: "4", wantCurr: "2", wantDetail: "NUMA nodes: expected 4, got 2"},
		{name: "spec 0, nodes 4", spec: 0, nodes: 4, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantSpec: "", wantCurr: "", wantDetail: "skipped: numa_nodes not set in spec"},
		{name: "spec -1, nodes 4", spec: -1, nodes: 4, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantDetail: "skipped: numa_nodes not set in spec"},
		{name: "spec 2, arch arm64, nodes 9", spec: 2, nodes: 9, arch: "arm64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantDetail: "skipped on arm64"},
		{name: "spec 0 unreadable sysfs", spec: 0, nodes: 4, arch: "amd64", info: gpus("NVIDIA H200"),
			wantStatus: consts.StatusNormal, wantDetail: "skipped: numa_nodes not set in spec"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the unreadable sysfs test, use nonexistent path
			if tt.name == "spec 0 unreadable sysfs" {
				useNumaFixture(t, filepath.Join(t.TempDir(), "nope"), tt.arch)
			} else {
				useNumaFixture(t, fakeNodeRoot(t, tt.nodes), tt.arch)
			}
			spec := &config.NvidiaSpec{Dependence: config.Dependence{NumaNodes: tt.spec}}
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
				assert.Contains(t, res.Suggestion, "ignored_checkers")
			}
		})
	}
}

func TestNumaNodesChecker_Errors(t *testing.T) {
	t.Run("wrong data type", func(t *testing.T) {
		useNumaFixture(t, fakeNodeRoot(t, 2), "amd64")
		c, err := NewNumaNodesChecker(&config.NvidiaSpec{})
		require.NoError(t, err)
		_, err = c.Check(context.Background(), "not nvidia info")
		assert.Error(t, err)
	})
	t.Run("sysfs unreadable", func(t *testing.T) {
		useNumaFixture(t, filepath.Join(t.TempDir(), "nope"), "amd64")
		c, err := NewNumaNodesChecker(&config.NvidiaSpec{Dependence: config.Dependence{NumaNodes: 2}})
		require.NoError(t, err)
		_, err = c.Check(context.Background(), gpus("NVIDIA H200"))
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
