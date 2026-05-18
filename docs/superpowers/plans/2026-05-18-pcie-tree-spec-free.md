# PCIe Tree Speed/Width spec-free 检查实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `PCIETreeSpeedDownDegraded` 和 `PCIETreeWidthIncorrect` 两个 checker 从"按 NIC `board_id` 查 HCA spec 拿 expect 值"改成"每条上行 PCIe link 取 `min(parent.max, self.max)` 从 sysfs 自描述判定"，多 NIC 混插场景下无需任何 spec 维护。

**Architecture:** Collector 一次扫描产出 `[]PCIETreeLink`（每条上行 link 一条记录，带 cur/max speed/width 及两端 BDF），Speed/Width 两个 checker 共用同一份数据做 per-link 比较；旧的 `PCIETreeSpeedMin*` / `PCIETreeWidthMin*` 字段保留并由 `PCIETreeLinks` 派生，保持下游 JSON/Prometheus 兼容。

**Tech Stack:** Go 1.23、`testify`、Linux sysfs（`/sys/bus/pci/devices/<BDF>/{current,max}_link_{speed,width}`）。

**Spec:** `docs/superpowers/specs/2026-05-18-pcie-tree-spec-free-design.md`
**Branch:** `feat/pcie-tree-spec-free` (`a9628d5`)

---

## File Structure

| 路径 | 操作 | 职责 |
| --- | --- | --- |
| `components/infiniband/collector/pcie_info.go` | Modify | `PCIPath` 转 var；新增 `PCIETreeLink` 结构、`GetPCIETreeLinks`、`getPCIETreeLinksByBDF`；删除旧 `GetPCIETreeMin` |
| `components/infiniband/collector/pcie_info_test.go` | Create | `GetPCIETreeLinks` 表驱动测试，用 `t.TempDir()` 伪造 sysfs |
| `components/infiniband/collector/ib_hardware_info.go` | Modify | `IBHardWareInfo` 增 `PCIETreeLinks`；`Collect` 调用新函数；新增 `minLinkCurSpeed`/`minLinkCurWidth` 派生旧 4 字段 |
| `components/infiniband/collector/ib_hardware_info_test.go` | Create | 验证 `minLinkCurSpeed`/`minLinkCurWidth` 派生正确 |
| `components/infiniband/checker/pcie_tree_speed.go` | Rewrite | 改用 `PCIETreeLinks` 做 per-link 比较；新增 `pcieSpeedLessThan` / `minNumericSpeed` 辅助 |
| `components/infiniband/checker/pcie_tree_speed_test.go` | Create | 表驱动 checker 测试 |
| `components/infiniband/checker/pcie_tree_width.go` | Rewrite | 与 speed 对称，复用辅助 |
| `components/infiniband/checker/pcie_tree_width_test.go` | Create | 表驱动 checker 测试 |
| `docs/infiniband.md` | Modify | 更新 PCIe Tree Speed/Width 检查的描述 |

---

## Conventions

- 测试用 `github.com/stretchr/testify/require`（错误中止）+ `github.com/stretchr/testify/assert`（断言）。
- 表驱动子用例：`t.Run(tc.name, func(t *testing.T) { ... })`。
- 新文件加上仓库标准的 Apache-2.0 Scitix 版权头（见 `CLAUDE.md`）。
- 提交信息：与最近 commit 风格一致（`Feat/...` 或动词开头），**不要** 加 `Co-Authored-By: Claude...` 或 `Generated with Claude Code` 尾巴。
- 每个 task 完成后 `go test ./components/infiniband/...` 至少跑通；最终 task 跑全量 `go test ./...`。

---

## Task 1: 让 `PCIPath` 可注入（var 改造 + 注入测试）

**Files:**
- Modify: `components/infiniband/collector/pcie_info.go:35-37`

- [ ] **Step 1: 把 `PCIPath` 从 const 改成 var**

Edit `components/infiniband/collector/pcie_info.go`：把

```go
const (
	PCIPath = "/sys/bus/pci/devices"
)
```

改为

```go
// PCIPath is the root of the PCI sysfs tree. It is a var (not a const) so
// tests can redirect it to a t.TempDir() before exercising the collector.
var PCIPath = "/sys/bus/pci/devices"
```

- [ ] **Step 2: 构建确认编译通过**

Run: `cd /root/devnet/sichek && go build ./components/infiniband/...`
Expected: `(no output)`，退出码 0。

- [ ] **Step 3: 提交**

```bash
git add components/infiniband/collector/pcie_info.go
git commit -m "Refactor/infiniband: make PCIPath injectable for tests"
```

---

## Task 2: 定义 `PCIETreeLink` 结构

**Files:**
- Modify: `components/infiniband/collector/pcie_info.go`（在 `PCIETreeInfo` 类型附近添加）

- [ ] **Step 1: 添加 `PCIETreeLink` 结构**

在 `pcie_info.go` 中 `PCIETreeWidthInfo` 类型定义之后（约 line 130 之后）追加：

```go
// PCIETreeLink represents a single PCIe link on the upstream path from an
// IB device to the root complex. Each adjacent (parent, child) BDF pair in
// the readlink path is one link. Speed/width strings keep their sysfs raw
// form (e.g. "32.0 GT/s PCIe" or "16") so downstream comparisons can choose
// how to parse them.
type PCIETreeLink struct {
	ParentBDF      string `json:"parent_bdf" yaml:"parent_bdf"`
	ChildBDF       string `json:"child_bdf" yaml:"child_bdf"`
	CurSpeed       string `json:"cur_speed" yaml:"cur_speed"`
	CurWidth       string `json:"cur_width" yaml:"cur_width"`
	ParentMaxSpeed string `json:"parent_max_speed" yaml:"parent_max_speed"`
	ChildMaxSpeed  string `json:"child_max_speed" yaml:"child_max_speed"`
	ParentMaxWidth string `json:"parent_max_width" yaml:"parent_max_width"`
	ChildMaxWidth  string `json:"child_max_width" yaml:"child_max_width"`
}
```

- [ ] **Step 2: 构建确认**

Run: `cd /root/devnet/sichek && go build ./components/infiniband/...`
Expected: `(no output)`，退出码 0。

- [ ] **Step 3: 提交**

```bash
git add components/infiniband/collector/pcie_info.go
git commit -m "Feat/infiniband: add PCIETreeLink struct"
```

---

## Task 3: 实现 `GetPCIETreeLinks` + 表驱动测试

**Files:**
- Create: `components/infiniband/collector/pcie_info_test.go`
- Modify: `components/infiniband/collector/pcie_info.go`

### Step 1: 写失败测试

> 关键测试基础设施约定：`getPCIETreeLinksByBDF` 只对 `<PCIPath>/<nicBDF>` 调用 `os.Readlink` 取 symlink 目标字符串；之后用正则提取目标里所有 BDF，并在 `<PCIPath>/<bdf>/` 下读 sysfs 文件。因此测试里：
> 1. 把每个上行 BDF（包括"看起来像 NIC 的"那个）写成 `<root>/<bdf>/` 真实目录 + 四个 sysfs 文件；
> 2. 在 `<root>/<entryBDF>` 创建一条 symlink（target 字符串内嵌路径 BDF 列表），并把 `entryBDF` 作为 NIC BDF 传入函数。`entryBDF` 用区别于真实 BDF 的字符串（如 `nic_entry_full`），避免和真实目录冲突。

- [ ] 创建 `components/infiniband/collector/pcie_info_test.go`：

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
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /root/devnet/sichek && go test ./components/infiniband/collector/ -run TestGetPCIETreeLinksByBDF -v`
Expected: 编译错误 `undefined: getPCIETreeLinksByBDF`。

### Step 3: 实现 `getPCIETreeLinksByBDF` 和 `GetPCIETreeLinks`

- [ ] 在 `components/infiniband/collector/pcie_info.go` 里追加（位置：放在旧的 `GetPCIETreeMin` 函数后面、`GetPCIETreeSpeed` 之前）：

```go
// GetPCIETreeLinks enumerates every PCIe link on the upstream path of an IB
// device. Each adjacent (parent, child) BDF pair in the readlink path becomes
// one link, with current_link_{speed,width} and max_link_{speed,width} read
// from both endpoints.  Direct-to-CPU devices (<2 BDFs in path) return nil.
//
// This is the spec-free replacement for GetPCIETreeMin: the checker compares
// link.CurSpeed against min(ParentMaxSpeed, ChildMaxSpeed) per link, so no
// HCA yaml expected value is needed.
func GetPCIETreeLinks(IBDev string) []PCIETreeLink {
	bdfList := GetIBDevBDF(IBDev)
	if len(bdfList) == 0 {
		logrus.WithField("component", "infiniband").Warnf("Could not get BDF for IB device %s", IBDev)
		return nil
	}
	return getPCIETreeLinksByBDF(bdfList[0])
}

// getPCIETreeLinksByBDF is the testable core of GetPCIETreeLinks. It does
// the readlink + per-link sysfs walk for a single NIC BDF.  Sysfs failures
// on individual files leave the corresponding fields blank rather than
// dropping the entire link, so the checker can still emit a useful message.
func getPCIETreeLinksByBDF(nicBDF string) []PCIETreeLink {
	devicePath := filepath.Join(PCIPath, nicBDF)
	linkPath, err := os.Readlink(devicePath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to resolve symlink for %s: %v", devicePath, err)
		return nil
	}

	bdfRegexPattern := `\b[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]\b`
	re := regexp.MustCompile(bdfRegexPattern)
	allBdfs := re.FindAllString(linkPath, -1)
	if len(allBdfs) < 2 {
		logrus.WithField("component", "infiniband").Infof("No upstream PCIe link for %s (path has <2 BDFs), skipping.", nicBDF)
		return nil
	}

	links := make([]PCIETreeLink, 0, len(allBdfs)-1)
	for i := 1; i < len(allBdfs); i++ {
		parent := allBdfs[i-1]
		child := allBdfs[i]
		link := PCIETreeLink{ParentBDF: parent, ChildBDF: child}
		link.CurSpeed = readSysfsString(filepath.Join(PCIPath, child, "current_link_speed"))
		if link.CurSpeed == "" {
			// Fall back to parent's report; the two endpoints share the link.
			link.CurSpeed = readSysfsString(filepath.Join(PCIPath, parent, "current_link_speed"))
		}
		link.CurWidth = readSysfsString(filepath.Join(PCIPath, child, "current_link_width"))
		if link.CurWidth == "" {
			link.CurWidth = readSysfsString(filepath.Join(PCIPath, parent, "current_link_width"))
		}
		link.ParentMaxSpeed = readSysfsString(filepath.Join(PCIPath, parent, "max_link_speed"))
		link.ChildMaxSpeed = readSysfsString(filepath.Join(PCIPath, child, "max_link_speed"))
		link.ParentMaxWidth = readSysfsString(filepath.Join(PCIPath, parent, "max_link_width"))
		link.ChildMaxWidth = readSysfsString(filepath.Join(PCIPath, child, "max_link_width"))
		links = append(links, link)
	}
	return links
}

// readSysfsString reads a single-line sysfs file and trims trailing whitespace.
// Returns "" on any read error; callers downgrade gracefully.
func readSysfsString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		logrus.WithField("component", "infiniband").Debugf("readSysfsString: %s: %v", path, err)
		return ""
	}
	return strings.TrimRight(string(data), "\n\r\t ")
}
```

- [ ] **Step 4: 确认 import 列表里有 `os`、`regexp`、`strings`、`path/filepath`、`logrus`**（这些应该已在文件顶部，不需要新加）。

- [ ] **Step 5: 跑测试确认通过**

Run: `cd /root/devnet/sichek && go test ./components/infiniband/collector/ -run TestGetPCIETreeLinksByBDF -v`
Expected: 所有 5 个子用例 PASS。

- [ ] **Step 6: 跑包内全部测试，确认没有回归**

Run: `cd /root/devnet/sichek && go test ./components/infiniband/collector/`
Expected: `ok`。

### Step 7: 提交

- [ ] 提交

```bash
git add components/infiniband/collector/pcie_info.go components/infiniband/collector/pcie_info_test.go
git commit -m "Feat/infiniband: add GetPCIETreeLinks spec-free collector"
```

---

## Task 4: 把 `PCIETreeLinks` 接进 `IBHardWareInfo`；旧 4 字段派生

**Files:**
- Modify: `components/infiniband/collector/ib_hardware_info.go`
- Create: `components/infiniband/collector/ib_hardware_info_test.go`

### Step 1: 加入字段

- [ ] Edit `ib_hardware_info.go`，在 `IBHardWareInfo` 结构中（line 58~61 之后）追加 `PCIETreeLinks` 字段：

```go
PCIETreeSpeedMin    string         `json:"pcie_tree_speed" yaml:"pcie_tree_speed"`
PCIETreeSpeedMinBDF string         `json:"pcie_tree_speed_bdf" yaml:"pcie_tree_speed_bdf"`
PCIETreeWidthMin    string         `json:"pcie_tree_width" yaml:"pcie_tree_width"`
PCIETreeWidthMinBDF string         `json:"pcie_tree_width_bdf" yaml:"pcie_tree_width_bdf"`
PCIETreeLinks       []PCIETreeLink `json:"pcie_tree_links" yaml:"pcie_tree_links"`
```

### Step 2: 改 `Collect` 走新函数 + 派生旧字段

- [ ] Edit `ib_hardware_info.go` 的 `Collect`，替换 line 122-123 的两行：

```go
hw.PCIETreeSpeedMin, hw.PCIETreeSpeedMinBDF = GetPCIETreeMin(IBDev, "current_link_speed")
hw.PCIETreeWidthMin, hw.PCIETreeWidthMinBDF = GetPCIETreeMin(IBDev, "current_link_width")
```

为：

```go
hw.PCIETreeLinks = GetPCIETreeLinks(IBDev)
hw.PCIETreeSpeedMin, hw.PCIETreeSpeedMinBDF = minLinkCurSpeed(hw.PCIETreeLinks)
hw.PCIETreeWidthMin, hw.PCIETreeWidthMinBDF = minLinkCurWidth(hw.PCIETreeLinks)
```

### Step 3: 写失败测试

- [ ] 创建 `components/infiniband/collector/ib_hardware_info_test.go`：

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
package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinLinkCurSpeed(t *testing.T) {
	cases := []struct {
		name     string
		links    []PCIETreeLink
		wantVal  string
		wantBDF  string
	}{
		{
			name:    "empty",
			links:   nil,
			wantVal: "",
			wantBDF: "",
		},
		{
			name: "single_link",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: "32.0 GT/s PCIe"},
			},
			wantVal: "32.0 GT/s PCIe",
			wantBDF: "b",
		},
		{
			name: "lowest_wins_returns_child_bdf",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: "32.0 GT/s PCIe"},
				{ParentBDF: "b", ChildBDF: "c", CurSpeed: "16.0 GT/s PCIe"},
				{ParentBDF: "c", ChildBDF: "d", CurSpeed: "8.0 GT/s PCIe"},
			},
			wantVal: "8.0 GT/s PCIe",
			wantBDF: "d",
		},
		{
			name: "blank_speeds_skipped",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: ""},
				{ParentBDF: "b", ChildBDF: "c", CurSpeed: "16.0 GT/s PCIe"},
			},
			wantVal: "16.0 GT/s PCIe",
			wantBDF: "c",
		},
		{
			name: "all_blank",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurSpeed: ""},
			},
			wantVal: "",
			wantBDF: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, bdf := minLinkCurSpeed(tc.links)
			assert.Equal(t, tc.wantVal, val)
			assert.Equal(t, tc.wantBDF, bdf)
		})
	}
}

func TestMinLinkCurWidth(t *testing.T) {
	cases := []struct {
		name    string
		links   []PCIETreeLink
		wantVal string
		wantBDF string
	}{
		{
			name:    "empty",
			links:   nil,
			wantVal: "",
			wantBDF: "",
		},
		{
			name: "lowest_wins",
			links: []PCIETreeLink{
				{ParentBDF: "a", ChildBDF: "b", CurWidth: "16"},
				{ParentBDF: "b", ChildBDF: "c", CurWidth: "8"},
			},
			wantVal: "8",
			wantBDF: "c",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, bdf := minLinkCurWidth(tc.links)
			assert.Equal(t, tc.wantVal, val)
			assert.Equal(t, tc.wantBDF, bdf)
		})
	}
}
```

- [ ] **Step 4: 跑测试确认失败**

Run: `cd /root/devnet/sichek && go test ./components/infiniband/collector/ -run "TestMinLinkCur" -v`
Expected: 编译错误 `undefined: minLinkCurSpeed`、`undefined: minLinkCurWidth`。

### Step 5: 实现派生函数

- [ ] 在 `ib_hardware_info.go` 文件末尾追加：

```go
// minLinkCurSpeed returns the smallest CurSpeed across links (using numeric
// comparison after extractNumericSpeed) and the ChildBDF of that link.
// Empty CurSpeed entries are skipped. Returns ("", "") if no link has a
// parseable speed.
func minLinkCurSpeed(links []PCIETreeLink) (string, string) {
	var (
		bestVal string
		bestBDF string
		bestNum float64
		found   bool
	)
	for _, l := range links {
		if l.CurSpeed == "" {
			continue
		}
		num, ok := parseNumericSpeed(l.CurSpeed)
		if !ok {
			continue
		}
		if !found || num < bestNum {
			bestNum = num
			bestVal = l.CurSpeed
			bestBDF = l.ChildBDF
			found = true
		}
	}
	return bestVal, bestBDF
}

// minLinkCurWidth is the width counterpart of minLinkCurSpeed.
func minLinkCurWidth(links []PCIETreeLink) (string, string) {
	var (
		bestVal string
		bestBDF string
		bestNum float64
		found   bool
	)
	for _, l := range links {
		if l.CurWidth == "" {
			continue
		}
		num, ok := parseNumericSpeed(l.CurWidth)
		if !ok {
			continue
		}
		if !found || num < bestNum {
			bestNum = num
			bestVal = l.CurWidth
			bestBDF = l.ChildBDF
			found = true
		}
	}
	return bestVal, bestBDF
}

// parseNumericSpeed reuses the same heuristic the checker uses:
// strip the unit suffix and parse the leading float.
func parseNumericSpeed(s string) (float64, bool) {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) == 0 {
		return 0, false
	}
	f, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
```

- [ ] **Step 6: 确认 import** — `ib_hardware_info.go` 顶部需要 `strconv` 和 `strings`；`strings` 已存在，若 `strconv` 缺失则手动添加。检查：

Run: `cd /root/devnet/sichek && head -30 components/infiniband/collector/ib_hardware_info.go`
Expected: 输出中 import 块包含 `strconv` 和 `strings`。如缺失，手动加 import。

### Step 7: 跑测试

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/collector/ -run "TestMinLinkCur" -v`
Expected: 7 个子用例 PASS。

- [ ] Run: `cd /root/devnet/sichek && go build ./components/infiniband/...`
Expected: (no output)，退出码 0。

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/...`
Expected: 全部 PASS（包括既有 checker，因为 `PCIETreeSpeedMin` 字段语义仍是"链路最小当前速度"，老 checker 的 spec 比较还能工作）。

### Step 8: 提交

- [ ] 提交

```bash
git add components/infiniband/collector/ib_hardware_info.go components/infiniband/collector/ib_hardware_info_test.go
git commit -m "Feat/infiniband: wire PCIETreeLinks into IBHardWareInfo, derive legacy fields"
```

---

## Task 5: Checker 共享比较辅助 `pcieSpeedLessThan` + `minNumericSpeed`

**Files:**
- Modify: `components/infiniband/checker/pcie_tree_speed.go`（追加 helper，**暂不动主逻辑**）
- Create: `components/infiniband/checker/pcie_helpers_test.go`

### Step 1: 写失败测试

- [ ] 创建 `components/infiniband/checker/pcie_helpers_test.go`：

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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPcieSpeedLessThan(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"less_simple", "16", "32", true},
		{"greater", "32", "16", false},
		{"equal_no_decimals", "16", "16", false},
		{"equal_with_decimals", "32.0", "32", false},
		{"less_gt_suffix", "16.0 GT/s PCIe", "32.0 GT/s PCIe", true},
		{"unparseable_a_returns_false", "abc", "32", false},
		{"unparseable_b_returns_false", "16", "xyz", false},
		{"empty_returns_false", "", "32", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, pcieSpeedLessThan(tc.a, tc.b))
		})
	}
}

func TestMinNumericSpeed(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want string
	}{
		{"a_smaller", "16.0 GT/s PCIe", "32.0 GT/s PCIe", "16.0 GT/s PCIe"},
		{"b_smaller", "32", "16", "16"},
		{"equal_returns_a", "32", "32", "32"},
		{"a_unparseable_returns_empty", "abc", "16", ""},
		{"b_unparseable_returns_empty", "16", "xyz", ""},
		{"both_empty_returns_empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, minNumericSpeed(tc.a, tc.b))
		})
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /root/devnet/sichek && go test ./components/infiniband/checker/ -run "TestPcieSpeedLessThan|TestMinNumericSpeed" -v`
Expected: 编译错误 `undefined: pcieSpeedLessThan`、`undefined: minNumericSpeed`。

### Step 3: 实现 helper

- [ ] 在 `components/infiniband/checker/pcie_tree_speed.go` 文件**末尾**追加（保留现有 `extractNumericSpeed`、`pcieSpeedEqual`、`numericSpeedEqual` 不动）：

```go
// pcieSpeedLessThan returns true iff a < b after extracting the leading numeric
// part of each (so "16.0 GT/s PCIe" parses as 16.0). Returns false when either
// value cannot be parsed — callers must treat "unknown" as "not less" so the
// checker stays normal on unreadable sysfs entries.
func pcieSpeedLessThan(a, b string) bool {
	af, errA := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(a)), 64)
	bf, errB := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(b)), 64)
	if errA != nil || errB != nil {
		return false
	}
	return af < bf-1e-9
}

// minNumericSpeed returns whichever of a or b parses to the smaller numeric
// value, preserving the raw string form. If either side is unparseable
// returns "" so the checker can skip the link rather than emit a noisy
// "unknown" comparison.
func minNumericSpeed(a, b string) string {
	af, errA := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(a)), 64)
	bf, errB := strconv.ParseFloat(strings.TrimSpace(extractNumericSpeed(b)), 64)
	if errA != nil || errB != nil {
		return ""
	}
	if af <= bf {
		return a
	}
	return b
}
```

### Step 4: 跑测试

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/checker/ -run "TestPcieSpeedLessThan|TestMinNumericSpeed" -v`
Expected: 14 个子用例全部 PASS。

### Step 5: 提交

- [ ] 提交

```bash
git add components/infiniband/checker/pcie_tree_speed.go components/infiniband/checker/pcie_helpers_test.go
git commit -m "Feat/infiniband: add pcieSpeedLessThan and minNumericSpeed helpers"
```

---

## Task 6: 重写 `IBPCIETreeSpeedChecker.Check`

**Files:**
- Rewrite: `components/infiniband/checker/pcie_tree_speed.go`（主逻辑部分）
- Create: `components/infiniband/checker/pcie_tree_speed_test.go`

### Step 1: 写失败测试

- [ ] 创建 `components/infiniband/checker/pcie_tree_speed_test.go`：

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
	"strings"
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildHW(dev, bdf string, links []collector.PCIETreeLink) collector.IBHardWareInfo {
	return collector.IBHardWareInfo{
		IBDev:         dev,
		PCIEBDF:       bdf,
		PCIETreeLinks: links,
	}
}

// runSpeedChecker constructs an InfinibandInfo from the given map of IBDev →
// IBHardWareInfo and runs the checker. The unexported sync.RWMutex inside
// InfinibandInfo is zero-value usable, so we don't touch it.
func runSpeedChecker(t *testing.T, hws map[string]collector.IBHardWareInfo) (string, string, string, string) {
	t.Helper()
	ck, err := NewIBPCIETreeSpeedChecker(&config.InfinibandSpec{})
	require.NoError(t, err)
	info := &collector.InfinibandInfo{IBHardWareInfo: hws}
	res, err := ck.Check(context.Background(), info)
	require.NoError(t, err)
	return res.Status, res.Device, res.Curr, res.Spec
}

func TestIBPCIETreeSpeedChecker_NoLinksReportsNormal(t *testing.T) {
	status, dev, curr, spec := runSpeedChecker(t, map[string]collector.IBHardWareInfo{
		"mlx5_0": buildHW("mlx5_0", "0000:82:00.0", nil),
	})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
	assert.Empty(t, curr)
	assert.Empty(t, spec)
}

func TestIBPCIETreeSpeedChecker_FullSpeedNormal(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
}

func TestIBPCIETreeSpeedChecker_RootGen4_NoAlarm(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "16.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, _, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
}

func TestIBPCIETreeSpeedChecker_OneLinkDegraded(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, curr, spec := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Contains(t, dev, "mlx5_0(0000:82:00.0, bottleneck@0000:81:00.0->0000:82:00.0)")
	assert.Equal(t, "16.0 GT/s PCIe", curr)
	assert.Equal(t, "32.0 GT/s PCIe", spec)
}

func TestIBPCIETreeSpeedChecker_MultipleLinksDegraded(t *testing.T) {
	hw := buildHW("mlx5_3", "0000:c1:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:c0:01.0", ChildBDF: "0000:c1:00.0",
			CurSpeed:       "8.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:bf:00.0", ChildBDF: "0000:c0:01.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, curr, spec := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_3": hw})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Equal(t, 2, strings.Count(dev, "bottleneck"))
	// Both currents and both caps joined by comma, order matches PCIETreeLinks order.
	assert.Equal(t, "8.0 GT/s PCIe,16.0 GT/s PCIe", curr)
	assert.Equal(t, "32.0 GT/s PCIe,32.0 GT/s PCIe", spec)
}

func TestIBPCIETreeSpeedChecker_MultiNIC_OneDegraded(t *testing.T) {
	good := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	bad := buildHW("mlx5_3", "0000:c1:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:c0:01.0", ChildBDF: "0000:c1:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{
		"mlx5_0": good,
		"mlx5_3": bad,
	})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Contains(t, dev, "mlx5_3")
	assert.NotContains(t, dev, "mlx5_0")
}

func TestIBPCIETreeSpeedChecker_UnparseableMaxSkipped(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "", ChildMaxSpeed: "Unknown",
		},
	})
	status, _, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /root/devnet/sichek && go test ./components/infiniband/checker/ -run "TestIBPCIETreeSpeedChecker" -v`
Expected: 测试不通过 —— 当前 checker 仍走旧 spec 比较，结构上行为不对。

### Step 3: 重写 checker 主逻辑

- [ ] 替换 `components/infiniband/checker/pcie_tree_speed.go` 中 **`Check` 方法**整段（line 72-150）为：

```go
func (c *IBPCIETreeSpeedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	hwInfoLen := len(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	if hwInfoLen == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	infinibandInfo.RLock()
	hws := uniqueByDev(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	failedDevices := make([]string, 0)
	failedCurr := make([]string, 0)
	failedCap := make([]string, 0)
	detailLines := make([]string, 0)
	suggestionLines := make([]string, 0)

	for _, hwInfo := range hws {
		if len(hwInfo.PCIETreeLinks) == 0 {
			// Direct-to-CPU or sysfs unavailable; treat as normal.
			continue
		}
		for _, link := range hwInfo.PCIETreeLinks {
			cap := minNumericSpeed(link.ParentMaxSpeed, link.ChildMaxSpeed)
			if cap == "" {
				// At least one endpoint's max is unparseable; skip silently
				// (collector already logged at debug).
				continue
			}
			if !pcieSpeedLessThan(link.CurSpeed, cap) {
				continue
			}
			result.Status = consts.StatusAbnormal
			devInfo := fmt.Sprintf("%s(%s, bottleneck@%s->%s)",
				hwInfo.IBDev, hwInfo.PCIEBDF, link.ParentBDF, link.ChildBDF)
			failedDevices = append(failedDevices, devInfo)
			failedCurr = append(failedCurr, link.CurSpeed)
			failedCap = append(failedCap, cap)
			detailLines = append(detailLines, fmt.Sprintf(
				"%s upstream link %s->%s current %s < cap %s",
				hwInfo.IBDev, link.ParentBDF, link.ChildBDF, link.CurSpeed, cap))
			suggestionLines = append(suggestionLines, fmt.Sprintf(
				"Check upstream PCIe link %s->%s for %s, current %s is below link capability %s (min of both endpoints' max).",
				link.ParentBDF, link.ChildBDF, hwInfo.IBDev, link.CurSpeed, cap))
		}
	}

	result.Curr = strings.Join(failedCurr, ",")
	result.Spec = strings.Join(failedCap, ",")
	result.Device = strings.Join(failedDevices, ",")
	if len(failedDevices) != 0 {
		result.Detail = strings.Join(detailLines, "\n")
		result.Suggestion = strings.Join(suggestionLines, "\n")
	}

	return &result, nil
}
```

- [ ] 同时**删除**该文件里 line 32-43 之间的 `pcieSpeedEqual` 函数（不再被引用）。**仅当 `go build` 后报 unused 错时再删，否则保留以减少改动半径。**

> 先不删，等所有 task 收尾时统一清理（Task 8）。

### Step 4: 跑测试

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/checker/ -run "TestIBPCIETreeSpeedChecker" -v`
Expected: 7 个子用例全部 PASS。

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/...`
Expected: 全部 PASS（width checker 仍是旧逻辑，但还能编译运行）。

### Step 5: 提交

- [ ] 提交

```bash
git add components/infiniband/checker/pcie_tree_speed.go components/infiniband/checker/pcie_tree_speed_test.go
git commit -m "Feat/infiniband: rewrite PCIETreeSpeedDownDegraded as per-link check"
```

---

## Task 7: 重写 `IBPCIETreeWidthChecker.Check`

**Files:**
- Rewrite: `components/infiniband/checker/pcie_tree_width.go`
- Create: `components/infiniband/checker/pcie_tree_width_test.go`

### Step 1: 写失败测试

- [ ] 创建 `components/infiniband/checker/pcie_tree_width_test.go`：

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
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runWidthChecker(t *testing.T, hws map[string]collector.IBHardWareInfo) (string, string, string, string) {
	t.Helper()
	ck, err := NewIBPCIETreeWidthChecker(&config.InfinibandSpec{})
	require.NoError(t, err)
	info := &collector.InfinibandInfo{IBHardWareInfo: hws}
	res, err := ck.Check(context.Background(), info)
	require.NoError(t, err)
	return res.Status, res.Device, res.Curr, res.Spec
}

func TestIBPCIETreeWidthChecker_NoLinksNormal(t *testing.T) {
	status, dev, _, _ := runWidthChecker(t, map[string]collector.IBHardWareInfo{
		"mlx5_0": {IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0", PCIETreeLinks: nil},
	})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
}

func TestIBPCIETreeWidthChecker_FullWidthNormal(t *testing.T) {
	hw := collector.IBHardWareInfo{
		IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0",
		PCIETreeLinks: []collector.PCIETreeLink{
			{
				ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
				CurWidth:       "16",
				ParentMaxWidth: "16", ChildMaxWidth: "16",
			},
		},
	}
	status, dev, _, _ := runWidthChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
}

func TestIBPCIETreeWidthChecker_OneLinkDegraded(t *testing.T) {
	hw := collector.IBHardWareInfo{
		IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0",
		PCIETreeLinks: []collector.PCIETreeLink{
			{
				ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
				CurWidth:       "8",
				ParentMaxWidth: "16", ChildMaxWidth: "16",
			},
		},
	}
	status, dev, curr, spec := runWidthChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Contains(t, dev, "mlx5_0(0000:82:00.0, bottleneck@0000:80:01.0->0000:82:00.0)")
	assert.Equal(t, "8", curr)
	assert.Equal(t, "16", spec)
}

func TestIBPCIETreeWidthChecker_ParentSlotWiderThanChild_NoAlarm(t *testing.T) {
	// E.g. slot is x16 but the NIC supports only x8; running at x8 is correct.
	hw := collector.IBHardWareInfo{
		IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0",
		PCIETreeLinks: []collector.PCIETreeLink{
			{
				ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
				CurWidth:       "8",
				ParentMaxWidth: "16", ChildMaxWidth: "8",
			},
		},
	}
	status, _, _, _ := runWidthChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /root/devnet/sichek && go test ./components/infiniband/checker/ -run "TestIBPCIETreeWidthChecker" -v`
Expected: 不通过（旧 checker 仍读 yaml）。

### Step 3: 重写 width checker

- [ ] 替换 `components/infiniband/checker/pcie_tree_width.go` 中 `Check` 方法整段（line 57-129）为：

```go
func (c *IBPCIETreeWidthChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	infinibandInfo.RLock()
	hwInfoLen := len(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	if hwInfoLen == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	infinibandInfo.RLock()
	hws := uniqueByDev(infinibandInfo.IBHardWareInfo)
	infinibandInfo.RUnlock()

	failedDevices := make([]string, 0)
	failedCurr := make([]string, 0)
	failedCap := make([]string, 0)
	detailLines := make([]string, 0)
	suggestionLines := make([]string, 0)

	for _, hwInfo := range hws {
		if len(hwInfo.PCIETreeLinks) == 0 {
			continue
		}
		for _, link := range hwInfo.PCIETreeLinks {
			cap := minNumericSpeed(link.ParentMaxWidth, link.ChildMaxWidth)
			if cap == "" {
				continue
			}
			if !pcieSpeedLessThan(link.CurWidth, cap) {
				continue
			}
			result.Status = consts.StatusAbnormal
			devInfo := fmt.Sprintf("%s(%s, bottleneck@%s->%s)",
				hwInfo.IBDev, hwInfo.PCIEBDF, link.ParentBDF, link.ChildBDF)
			failedDevices = append(failedDevices, devInfo)
			failedCurr = append(failedCurr, link.CurWidth)
			failedCap = append(failedCap, cap)
			detailLines = append(detailLines, fmt.Sprintf(
				"%s upstream link %s->%s current width x%s < cap x%s",
				hwInfo.IBDev, link.ParentBDF, link.ChildBDF, link.CurWidth, cap))
			suggestionLines = append(suggestionLines, fmt.Sprintf(
				"Check upstream PCIe link %s->%s for %s, current width x%s is below link capability x%s (min of both endpoints' max).",
				link.ParentBDF, link.ChildBDF, hwInfo.IBDev, link.CurWidth, cap))
		}
	}

	result.Curr = strings.Join(failedCurr, ",")
	result.Spec = strings.Join(failedCap, ",")
	result.Device = strings.Join(failedDevices, ",")
	if len(failedDevices) != 0 {
		result.Detail = strings.Join(detailLines, "\n")
		result.Suggestion = strings.Join(suggestionLines, "\n")
	}

	return &result, nil
}
```

- [ ] 确认 `pcie_tree_width.go` 顶部 import 包含 `strings`、`fmt`、`common`、`collector`、`config`、`consts`、`logrus`（多数已存在；`logrus` 不再用可删——但若编译报 unused 再删）。

### Step 4: 跑测试

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/checker/ -run "TestIBPCIETreeWidthChecker" -v`
Expected: 4 个子用例 PASS。

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/...`
Expected: 全部 PASS。

### Step 5: 提交

- [ ] 提交

```bash
git add components/infiniband/checker/pcie_tree_width.go components/infiniband/checker/pcie_tree_width_test.go
git commit -m "Feat/infiniband: rewrite PCIETreeWidthIncorrect as per-link check"
```

---

## Task 8: 删除已废弃的 `GetPCIETreeMin`、清理未引用 helper

**Files:**
- Modify: `components/infiniband/collector/pcie_info.go`
- Modify: `components/infiniband/checker/pcie_tree_speed.go`（清理未引用的 `pcieSpeedEqual` / `numericSpeedEqual`）
- Modify: `components/infiniband/checker/pcie_tree_width.go`（清理未引用的 `logrus` import 等）

### Step 1: 确认无外部引用

- [ ] Run: `cd /root/devnet/sichek && grep -rn "GetPCIETreeMin\b" --include="*.go" | grep -v _test.go`
Expected: 仅 `components/infiniband/collector/pcie_info.go` 内部。

- [ ] Run: `cd /root/devnet/sichek && grep -rn "pcieSpeedEqual\b\|numericSpeedEqual\b" --include="*.go" | grep -v _test.go`
Expected: 仅在 `pcie_tree_speed.go` 内部定义；checker 主体已不再调用。

### Step 2: 删除函数与未引用 helper

- [ ] 从 `components/infiniband/collector/pcie_info.go` 删除 `GetPCIETreeMin`（line 234-329 整段含 doc 注释）。

- [ ] 从 `components/infiniband/checker/pcie_tree_speed.go` 删除（line 31-43 的 `pcieSpeedEqual` 和 line 163-171 的 `numericSpeedEqual`），保留 `extractNumericSpeed`（仍被 helpers 使用）以及新加的 `pcieSpeedLessThan`、`minNumericSpeed`。

### Step 3: 修复任何编译错误（unused import 等）

- [ ] Run: `cd /root/devnet/sichek && go build ./components/infiniband/...`
Expected: 退出码 0。如有 `imported and not used` 报错，删除对应 import 行。常见受影响 import：
  - `pcie_info.go`：`math`（仅 `GetPCIETreeMin` 用过），删除 `math` import。
  - `pcie_tree_width.go`：旧版可能有 `logrus`，新版若未使用则删除该 import 行。

### Step 4: 全量测试

- [ ] Run: `cd /root/devnet/sichek && go test ./components/infiniband/...`
Expected: ok。

- [ ] Run: `cd /root/devnet/sichek && go vet ./components/infiniband/...`
Expected: (no output)。

### Step 5: 提交

- [ ] 提交

```bash
git add components/infiniband/collector/pcie_info.go components/infiniband/checker/pcie_tree_speed.go components/infiniband/checker/pcie_tree_width.go
git commit -m "Refactor/infiniband: drop legacy GetPCIETreeMin and unused checker helpers"
```

---

## Task 9: 全量构建 + 单测回归

**Files:** (none)

- [ ] **Step 1: 全量构建**

Run: `cd /root/devnet/sichek && make`
Expected: `build/bin/sichek` 生成；构建无 warning。

- [ ] **Step 2: 全量单测**

Run: `cd /root/devnet/sichek && go test ./...`
Expected: 所有包 `ok`。

- [ ] **Step 3: vet**

Run: `cd /root/devnet/sichek && go vet ./...`
Expected: (no output)。

- [ ] **Step 4: 如果 Makefile / go.mod 有未提交的非本任务改动**

不在本计划范围；保留为 working tree 改动，不提交。

> 此 Task 不产生 commit。

---

## Task 10: 更新 `docs/infiniband.md`

**Files:**
- Modify: `docs/infiniband.md`

### Step 1: 找到对应章节

- [ ] Run: `grep -n -i "pcie.tree\|treespeed\|treewidth\|PCIETreeSpeed\|PCIETreeWidth" /root/devnet/sichek/docs/infiniband.md`
Expected: 若返回行号 N，则在 N 附近做修改；若返回空，则文末添加新小节"PCIe Tree Speed/Width"。

### Step 2: 在对应位置追加 / 替换一段说明（中文，与现有文档风格一致）

- [ ] 添加（或替换）如下段落：

```markdown
## PCIe Tree Speed/Width 检测

`PCIETreeSpeedDownDegraded` / `PCIETreeWidthIncorrect` 不再依赖 HCA `board_id` spec
中的 `pcie_tree_speed` / `pcie_tree_width` 字段。Collector 直接从 sysfs
（`/sys/bus/pci/devices/<BDF>/{current,max}_link_{speed,width}`）枚举从 NIC 到 root
complex 的每一条 PCIe link，对每条 link 取两端 `max_link_*` 的较小者作为该链路
理论上限；当 `current_link_*` 低于该上限时上报。

效果：

- 多 NIC 混插场景无需补 spec；新 SKU 上线零维护。
- 当 root port 物理上限低于 NIC（例如 root port 仅支持 Gen4 而 NIC 是 Gen5）时
  不再误报。
- `Detail` / `Suggestion` 字段精确到出问题的具体 link，例如 `bottleneck@0000:80:01.0->0000:81:00.0`。

`pcie_tree_speed` / `pcie_tree_width` 字段在 HCA spec yaml 中保留为历史信息，
运行时不再读取。
```

### Step 3: 提交

- [ ] 提交（用 `-f` 因为 `*.md` 在 .gitignore 中，但 `docs/` 下的现有文件已被跟踪——如果是修改而非新增不需要 `-f`）：

```bash
git add docs/infiniband.md
git commit -m "Docs/infiniband: explain spec-free PCIe Tree Speed/Width check"
```

---

## Summary

10 个 task：
- Task 1-4：collector 侧（注入点 + 新结构 + 派生）；
- Task 5：共享辅助函数；
- Task 6-7：两个 checker 重写；
- Task 8：清理废弃代码；
- Task 9：全量回归；
- Task 10：文档。

每个 task 自带表驱动测试 + 单独 commit；TDD 顺序明确（先红后绿再重构）。落地后 `docs/superpowers/specs/2026-05-18-pcie-tree-spec-free-design.md` 中所有"目标"都有对应任务覆盖。
