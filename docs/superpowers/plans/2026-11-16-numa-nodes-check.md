# NUMA 节点数检查 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 nvidia 组件新增 `numa_nodes` dependence checker：按 GPU 型号检查 OS 可见的 NUMA 节点数（B300 → 4，其他 → 2），用来发现 BIOS SNC/NPS 配置漂移。

**Architecture:** 新增一个 checker 文件，放在 `components/nvidia/checker/check_dependences/`，和 `iommu.go` 同包。期望值来自代码内置规则，spec 的 `dependence.numa_nodes` 可选覆盖。实际值通过数 `/sys/devices/system/node/node[0-9]+` 目录得到。checker 在 `checker.go` 的构造表里注册后，daemon 默认会跑。

**Tech Stack:** Go 1.23，testify (`assert`/`require`)，vendored deps。

**Spec:** `docs/superpowers/specs/2026-11-16-numa-nodes-check-design.md`

## Global Constraints

- 分支 `feat/numa-nodes-check`。依赖都在 vendor 里，不新增依赖。
- 新文件带 Apache-2.0 Scitix 版权头（见 CLAUDE.md）。
- checker 名 `numa_nodes`，ErrorName `NUMANodeCountMismatch`，Level `consts.LevelWarning`。
- 内置规则：GPU 名称包含 `B300`（不区分大小写）期望 4，其他期望 2。
- `Dependence.NumaNodes` 的 tag 是 `json:"numa_nodes,omitempty" yaml:"numa_nodes,omitempty"`。值 `> 0` 时覆盖内置规则，否则用内置规则。
- 非 amd64 平台跳过（Normal）；拿不到 GPU 名称时跳过（Normal）；读 sysfs 失败或数出 0 个节点时返回 error。
- 不改 cpu 组件、pcie_topo、annotation schema、metrics、sichek_spec。
- 提交信息不要带 `Co-Authored-By: Claude` 或 "Generated with Claude Code"（用户 memory 规则）。
- `.gitignore` 忽略 `*.md`，提交本计划或文档时要 `git add -f`。

## Review Focus

1. **spec 重新序列化不能写出 `numa_nodes: 0`**：sichek 会改写 `default_spec.yaml`。没配置的集群在 yaml marshal 之后不应该出现这个键。由 Task 1 的 `TestDependenceNumaNodesOmitEmpty` 覆盖。
2. **第一张 GPU 名称为空、后面的有名称**：比如第一张卡采集失败。这时应该用第一个非空名称，不能直接跳过。由 Task 1 的 `TestNumaNodesChecker_Check` 的 "first device name empty" 用例覆盖。
3. **spec 配了负数**：`numa_nodes: -1` 应该当成没配置，回落到内置规则，不能拿 -1 去比较。由 Task 1 的 "negative override ignored" 用例覆盖。
4. **sysfs 节点目录里混有非节点条目**：`has_cpu`、`online`、`possible`、`power` 等不能算进去，`node` 后面不是纯数字的也不能算。由 Task 1 的 `TestCountNumaNodes` 覆盖。
5. **user config 的 `ignored_checkers: [numa_nodes]` 能关掉检查**：走的是现有机制。Task 2 验证 checker 名称和 GPUCheckItems 的 key 一致，这个机制就能生效。

---

### Task 1: `numa_nodes` checker（spec 字段 + check item + checker + 单测）

**Files:**
- Modify: `components/nvidia/config/spec.go`（`Dependence` struct，约第 64-70 行）
- Modify: `components/nvidia/config/check_items.go`（常量块约第 25 行附近；`GPUCheckItems` map 约第 51 行起）
- Create: `components/nvidia/checker/check_dependences/numa_nodes.go`
- Create: `components/nvidia/checker/check_dependences/numa_nodes_test.go`

**Interfaces:**
- Consumes: `collector.NvidiaInfo{DevicesInfo []collector.DeviceInfo}`，其中 `DeviceInfo.Name string`；`config.NvidiaSpec.Dependence`；`config.GPUCheckItems map[string]common.CheckerResult`
- Produces:
  - `config.NumaNodesCheckerName = "numa_nodes"`
  - `config.Dependence.NumaNodes int`
  - `func NewNumaNodesChecker(cfg *config.NvidiaSpec) (common.Checker, error)`（package `checker`，目录 `check_dependences`，被 `checker.go` 以别名 `dependence` 导入）
  - 包内函数：`expectedNumaNodes(gpuName string) int`、`countNumaNodes(root string) (int, error)`；包级变量 `numaSysfsRoot string`、`numaCheckArch string`

- [ ] **Step 1: 加 spec 字段和 check item**

`components/nvidia/config/spec.go` 的 `Dependence` 改成：

```go
type Dependence struct {
	PcieAcs        string `json:"pcie_acs" yaml:"pcie_acs"`
	Iommu          string `json:"iommu" yaml:"iommu"`
	NvidiaPeermem  string `json:"nv_peermem" yaml:"nv_peermem"`
	FabricManager  string `json:"nv_fabricmanager" yaml:"nv_fabricmanager"`
	CpuPerformance string `json:"cpu_performance" yaml:"cpu_performance"`
	// NumaNodes overrides the built-in expected NUMA node count (B300 → 4, others → 2).
	// Zero or negative means "use the built-in default".
	NumaNodes int `json:"numa_nodes,omitempty" yaml:"numa_nodes,omitempty"`
}
```

`components/nvidia/config/check_items.go`：在 `IOMMUCheckerName = "iommu"` 那一行下面加常量（和周围对齐）：

```go
	NumaNodesCheckerName                 = "numa_nodes"
```

在 `GPUCheckItems` 的 `IOMMUCheckerName: {...},` 条目后面加：

```go
	NumaNodesCheckerName: {
		Name:        NumaNodesCheckerName,
		Description: "Check NUMA node count matches the expected BIOS SNC/NPS setting for this GPU model",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "NUMA node count matches expected configuration",
		ErrorName:   "NUMANodeCountMismatch",
		Suggestion:  "Check BIOS Sub-NUMA Clustering (SNC) / AMD NPS setting; a reboot is required after the change",
	},
```

- [ ] **Step 2: 写失败的测试**

创建 `components/nvidia/checker/check_dependences/numa_nodes_test.go`：

```go
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
```

注意：`fakeNodeRoot` 用 `string(rune('0'+i))` 生成 node0..node9，测试里最多用到 9 个节点，不会超出范围。yaml 库用 `sigs.k8s.io/yaml`，和 `components/common/spec.go` 加载、改写 spec 时用的一致。它的 Marshal 走 json tag，所以要验证的是 json tag 上的 `omitempty`。

- [ ] **Step 3: 跑测试，确认失败**

Run: `go test ./components/nvidia/checker/check_dependences/ -run 'Numa' -v`
Expected: 编译失败，报 `undefined: numaSysfsRoot` / `undefined: NewNumaNodesChecker` 等。

- [ ] **Step 4: 写实现**

创建 `components/nvidia/checker/check_dependences/numa_nodes.go`：

```go
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
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

// Overridable in tests.
var (
	numaSysfsRoot = "/sys/devices/system/node"
	numaCheckArch = runtime.GOARCH
)

var numaNodeDirRe = regexp.MustCompile(`^node[0-9]+$`)

type NumaNodesChecker struct {
	name string
	cfg  *config.NvidiaSpec
}

func NewNumaNodesChecker(cfg *config.NvidiaSpec) (common.Checker, error) {
	return &NumaNodesChecker{
		name: config.NumaNodesCheckerName,
		cfg:  cfg,
	}, nil
}

func (c *NumaNodesChecker) Name() string {
	return c.name
}

// expectedNumaNodes is the built-in BIOS SNC/NPS baseline per GPU model:
// B300 platforms run SNC2 (4 nodes on 2 sockets), everything else 2.
func expectedNumaNodes(gpuName string) int {
	if strings.Contains(strings.ToUpper(gpuName), "B300") {
		return 4
	}
	return 2
}

func countNumaNodes(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if numaNodeDirRe.MatchString(e.Name()) {
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("no NUMA node entries under %s", root)
	}
	return n, nil
}

// Check compares the OS-visible NUMA node count against the expected count for this GPU model.
func (c *NumaNodesChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected NvidiaInfo")
	}
	result := config.GPUCheckItems[config.NumaNodesCheckerName]

	// Grace (GB200/GB300) exposes GPU HBM as NUMA nodes, so the count is meaningless there.
	if numaCheckArch != "amd64" {
		result.Detail = fmt.Sprintf("skipped on %s", numaCheckArch)
		return &result, nil
	}

	gpuName := ""
	for _, d := range info.DevicesInfo {
		if d.Name != "" {
			gpuName = d.Name
			break
		}
	}
	if gpuName == "" {
		result.Detail = "skipped: GPU name unavailable"
		return &result, nil
	}

	actual, err := countNumaNodes(numaSysfsRoot)
	if err != nil {
		return nil, fmt.Errorf("count NUMA nodes: %w", err)
	}

	expected := expectedNumaNodes(gpuName)
	source := "default for " + gpuName
	if c.cfg != nil && c.cfg.Dependence.NumaNodes > 0 {
		expected = c.cfg.Dependence.NumaNodes
		source = "spec override"
	}

	result.Spec = strconv.Itoa(expected)
	result.Curr = strconv.Itoa(actual)
	if actual != expected {
		logrus.WithFields(logrus.Fields{
			"checker":  c.Name(),
			"expected": expected,
			"curr":     actual,
			"source":   source,
		}).Errorf("NUMA node count mismatch")
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("NUMA nodes: expected %d (%s), got %d", expected, source, actual)
		result.Suggestion = fmt.Sprintf("Check BIOS Sub-NUMA Clustering (SNC) / AMD NPS setting; this GPU model expects %d NUMA node(s). A reboot is required after the change.", expected)
		return &result, nil
	}
	result.Detail = fmt.Sprintf("NUMA nodes: %d (%s)", actual, source)
	result.Suggestion = ""
	return &result, nil
}
```

- [ ] **Step 5: 跑测试，确认通过**

Run: `go test ./components/nvidia/checker/check_dependences/ -run 'Numa' -v && go test ./components/nvidia/config/...`
Expected: 全部 PASS。

- [ ] **Step 6: 提交**

```bash
git add components/nvidia/config/spec.go components/nvidia/config/check_items.go \
  components/nvidia/checker/check_dependences/numa_nodes.go \
  components/nvidia/checker/check_dependences/numa_nodes_test.go
git commit -m "feat(nvidia): add numa_nodes dependence checker (B300=4, others=2)"
```

---

### Task 2: 注册 checker 并整包验证

**Files:**
- Modify: `components/nvidia/checker/checker.go`（`checkerConstructors` map，约第 29-52 行）
- Test: `components/nvidia/checker/checker_test.go`（只追加一个测试函数）

**Interfaces:**
- Consumes: Task 1 的 `dependence.NewNumaNodesChecker`、`config.NumaNodesCheckerName`
- Produces: 无（`NewCheckers` 的返回值里多出 `numa_nodes`）

- [ ] **Step 1: 写失败的测试**

在 `components/nvidia/checker/checker_test.go`（包名 `checker`）末尾追加：

```go
func TestNumaNodesCheckerRegistered(t *testing.T) {
	checkers, err := NewCheckers(&config.NvidiaUserConfig{Nvidia: &config.NvidiaConfig{}}, &config.NvidiaSpec{})
	assert.NoError(t, err)
	names := make([]string, 0, len(checkers))
	for _, c := range checkers {
		names = append(names, c.Name())
	}
	assert.Contains(t, names, config.NumaNodesCheckerName)

	ignored, err := NewCheckers(&config.NvidiaUserConfig{Nvidia: &config.NvidiaConfig{IgnoredCheckers: []string{config.NumaNodesCheckerName}}}, &config.NvidiaSpec{})
	assert.NoError(t, err)
	for _, c := range ignored {
		assert.NotEqual(t, config.NumaNodesCheckerName, c.Name())
	}
}
```

类型已经核对过：`NvidiaUserConfig{Nvidia *NvidiaConfig}`，`NvidiaConfig.IgnoredCheckers []string`。`checker_test.go` 已经导入 `config`，如果缺 `assert` 就补上 `github.com/stretchr/testify/assert`。`NewCheckers` 内部调用的 `nvutils.GetComputeCapability(0)` 在没有 GPU 的开发机上会失败，但只会打 Warn 日志、跳过 IBGDA，不影响这个测试。如果在开发机上它 panic 而不是返回 error，就给测试加 `t.Skip` 守卫，并在报告里说明。

- [ ] **Step 2: 跑测试，确认失败**

Run: `go test ./components/nvidia/checker/ -run TestNumaNodesCheckerRegistered -v`
Expected: FAIL，`names` 里不包含 `numa_nodes`。

- [ ] **Step 3: 注册**

在 `checkerConstructors` map 里 `config.IOMMUCheckerName: dependence.NewIOMMUChecker,` 那一行下面加（gofmt 会对齐）：

```go
		config.NumaNodesCheckerName:                 dependence.NewNumaNodesChecker,
```

- [ ] **Step 4: 跑测试，再做全量验证**

Run:
```bash
go test ./components/nvidia/checker/ -run TestNumaNodesCheckerRegistered -v
gofmt -l components/nvidia/
go vet ./components/nvidia/...
go test ./components/nvidia/...
go build ./...
```
Expected: 新测试 PASS；`gofmt -l` 没有输出；vet 和 build 通过。`go test ./components/nvidia/...` 里如果有需要真 GPU 的旧测试，在开发机上失败，先在 main 上跑同样的命令，确认是本来就失败的，再在报告里列出来。

- [ ] **Step 5: 提交**

```bash
git add components/nvidia/checker/checker.go components/nvidia/checker/checker_test.go
git commit -m "feat(nvidia): register numa_nodes checker"
```

---

### Task 3: 现场回归

由主会话用 `sichek-field-regression` skill 来做，不交给 implementer 子代理（要 SSH 到真机）。

- [ ] **Step 1:** 用 `make` 构建 linux/amd64，分发到 `/tmp/sichek-test`。测试时把 `SICHEK_CONFIG_DIR` 指到 /tmp 下的副本，避免覆盖生产 config/spec。
- [ ] **Step 2:** 在各节点跑 `SICHEK_CONFIG_DIR=/tmp/sk-cfg /tmp/sichek-test gpu -v`（或 `sichek all` 的 nvidia 段），检查 `numa_nodes` 的结果：

| 节点 | 期望 |
|---|---|
| root@214.196.33.97 | Abnormal / warning，`expected 2 (default for NVIDIA H200), got 4` |
| shg06 | Abnormal / warning，expected 2，got 4 |
| root@214.196.33.98、clnet36、lmg86、dracog24、thg1、shg205 | Normal |
| root@10.220.77.49（B300） | Normal，expected 4，got 4 |

- [ ] **Step 3:** 确认生产 `/var/sichek/config/default_spec.yaml` 的 md5 前后一致。把结果写进回归报告。
