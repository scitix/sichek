# server_model_id 上报 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** sichek daemon 启动 5 分钟后、之后每 1 小时运行硬件团队的 catcher 脚本，把机型ID（`server_model_id`）与明细写进 snapshot（随 reporter 上报）并导出 Prometheus 指标。

**Architecture:** catcher 源码随仓库打包到 `/var/sichek/scripts/catcher/`。`service/servermodel.go` 负责：复制脚本到临时目录 → 以独立进程组执行 `catcher.bash json`（超时杀整组）→ 解析 `result/catcher_result.json` → 计算 complete/missing → 写入 `SnapshotManager` 并更新指标。失败保留上次成功值，只更新 `error`。不新增 component。

**Tech Stack:** Go 1.23（vendored），`os/exec` + `syscall`，`gopkg.in/yaml.v3`，`prometheus/client_golang`，testify。

**Spec:** `docs/superpowers/specs/2026-11-13-server-model-id-design.md`

## Global Constraints

- 分支 `feat/server-model-id`（已基于 main 创建）。
- 新 Go 文件带 Apache-2.0 Scitix 版权头（见 CLAUDE.md）。
- snapshot 顶层 JSON 键：`server_model_id`（string）、`server_model`（对象），两者 `omitempty`。
- `server_model` 字段：`complete`、`missing`、`components`(cpu/gpu/nic/mem/disk → `code`/`detected`/`key`)、`config`、`catcher_version`、`collected_at`、`error`。
- 指标：`sichek_server_model_info{server_model_id, complete, node} 1`，每节点只保留一条。
- 配置块 `server_model:` 默认 `enable: true`、`script_path: /var/sichek/scripts/catcher/catcher.bash`、`initial_delay: 5m`、`interval: 1h`、`timeout: 120s`。
- catcher 版本：tar sha256 `021a25d67602ea35cc8f74cfdf675a843ab8aa2daa746b0299da57c961ecea31`，拉取日期 2026-11-13。
- 仓库 `.gitignore` 含 `*.md` 与 `*.html`：catcher 的 `README.md`/`CLAUDE.md`/`docs/MATCHING.md` 及本计划提交时需 `git add -f`。
- commit 消息**不加**任何 Claude/AI Co-Authored-By 尾注（用户 memory 规定）。

## Review Focus

1. catcher 退出码 0 但 `result/catcher_result.json` 不存在（如 `json` 子命令内部 mkdir 失败）→ 应记为 error 并保留旧值，而不是 panic 或清空 ID。（Task 2 测试 `TestRunCatcher_NoResultFile`）
2. catcher 后台留下孙进程握住 stdout 管道 → 超时后必须按时返回，不能永久阻塞。（Task 2 测试 `TestRunCatcher_TimeoutKillsProcessGroup`）
3. 第一次运行就失败（无旧值）→ snapshot 仍写入 `server_model.error`，`server_model_id` 为空且被省略，指标不写。（Task 3 测试 `TestServerModelRunner_FirstRunFailure`）
4. 无 GPU 的 CPU 节点 gpu 段为空 → 不能被标成 missing。（Task 1 测试表中 `no gpu present` 用例）
5. config 文件里写了 `server_model:` 但只填了部分字段（或 duration 为 0）→ 其余字段回落到默认值，不能出现 interval=0 导致 ticker panic。（Task 3 测试 `TestLoadServerModelConfig`）

---

## File Structure

| 文件 | 职责 |
|------|------|
| `scripts/catcher/**`（新增） | catcher 原样源码 + `VERSION` |
| `service/servermodel.go`（新增） | 类型定义、配置加载、结果解析/计算、执行 catcher、定时 runner |
| `service/servermodel_test.go`（新增） | 上述单测 |
| `service/snapshot.go` | `Snapshot` 新增两字段；`SetServerModel()` |
| `service/snapshot_test.go` | `SetServerModel` 测试 |
| `metrics/servermodel.go`（新增） | `ServerModelInfo` gauge 及 `SetServerModelInfo()` |
| `metrics/servermodel_test.go`（新增） | 指标替换测试 |
| `service/daemon.go` | 加载配置、构造并启动 runner |
| `config/default_user_config.yaml` | `server_model:` 默认块 |
| `docker/Dockerfile`、`Dockerfile.cuda128`、`Dockerfile.cuda130`、`Dockerfile.centos8` | 运行时装 `dmidecode`、`ethtool` |

---

### Task 0: 引入 catcher 源码

**Files:**
- Create: `scripts/catcher/`（catcher.bash、lib/*.sh、docs/*、README.md、CLAUDE.md、.gitignore）
- Create: `scripts/catcher/VERSION`

**Interfaces:**
- Produces: 安装后脚本路径 `/var/sichek/scripts/catcher/catcher.bash`；版本文件 `/var/sichek/scripts/catcher/VERSION`（单行文本）。

- [ ] **Step 1: 下载并校验**

```bash
D=$(mktemp -d) && cd $D && wget -q https://oss-cn-shanghai.siflow.cn/scitix-release/catcher.tar.gz
echo "021a25d67602ea35cc8f74cfdf675a843ab8aa2daa746b0299da57c961ecea31  catcher.tar.gz" | sha256sum -c
```
Expected: `catcher.tar.gz: OK`。若不一致，说明 OSS 上已更新——停下来告知用户，不要继续。

- [ ] **Step 2: 解压到仓库**

```bash
cd /root/devnet/sichek
mkdir -p scripts/catcher && tar -xzf $D/catcher.tar.gz -C scripts/catcher --strip-components=1
chmod +x scripts/catcher/catcher.bash
printf 'sha256:021a25d67602ea35cc8f74cfdf675a843ab8aa2daa746b0299da57c961ecea31 2026-11-13\n' > scripts/catcher/VERSION
ls -R scripts/catcher
```
Expected: 有 `catcher.bash`、`lib/{catalog,detect,idgen,get-net}.sh`、`docs/{MATCHING.md,nic-spec.txt}`、`VERSION`。

- [ ] **Step 3: 本机冒烟（describe 不碰硬件）**

```bash
bash scripts/catcher/catcher.bash describe I57G90N13M63S10
```
Expected: 打印 `CPU: Intel 6530 *2` 等配置行。

- [ ] **Step 4: Commit**

```bash
git add scripts/catcher
git add -f scripts/catcher/README.md scripts/catcher/CLAUDE.md scripts/catcher/docs/MATCHING.md
git status --short scripts/catcher   # 确认全部文件已暂存
git commit -m "chore: vendor hardware team's catcher script (sha256 021a25d6)"
```

---

### Task 1: 类型定义与结果解析

**Files:**
- Create: `service/servermodel.go`
- Create: `service/servermodel_test.go`

**Interfaces:**
- Produces:
  - `type ServerModelPart struct { Code, Detected, Key string }`（json: `code`/`detected`/`key`）
  - `type ServerModel struct { Complete bool; Missing []string; Components map[string]ServerModelPart; Config string; CatcherVersion string; CollectedAt time.Time; Error string }`
  - `type catcherResult struct { ModelID string; Components map[string]ServerModelPart; Config string }`
  - `func parseCatcherResult(data []byte) (string, *ServerModel, error)` —— 返回 (model_id, 明细, err)；`CollectedAt`/`CatcherVersion` 由调用方填。

- [ ] **Step 1: 写失败测试**

`service/servermodel_test.go`：

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
package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const catcherFull = `{"model_id":"I57G90N13M63S10","components":{
 "cpu":{"code":"I57","detected":"INTEL(R) XEON(R) GOLD 6530 (2 socket)","key":"Intel 6530|2"},
 "gpu":{"code":"G90","detected":"NVIDIA GeForce RTX 5090 *8","key":"RTX5090|8"},
 "nic":{"code":"N13","detected":"100Gbps*2","key":"100Gx2*1"},
 "mem":{"code":"M63","detected":"DDR5 64G *16","key":"64|16"},
 "disk":{"code":"S10","detected":"SSD 960G*2 + SSD 3.84T*2","key":"960G*2+3.84T*2"}},
 "config":"CPU: Intel 6530 *2\nGPU: 5090 *8"}`

const catcherMissingDisk = `{"model_id":"I57G90N13M63","components":{
 "cpu":{"code":"I57","detected":"x","key":"Intel 6530|2"},
 "gpu":{"code":"G90","detected":"x","key":"RTX5090|8"},
 "nic":{"code":"N13","detected":"x","key":"100Gx2*1"},
 "mem":{"code":"M63","detected":"x","key":"64|16"},
 "disk":{"code":"","detected":"SSD 894G*1","key":"894G*1"}},"config":""}`

const catcherNoGPU = `{"model_id":"I57N13M63S10","components":{
 "cpu":{"code":"I57","detected":"x","key":"k"},
 "gpu":{"code":"","detected":"","key":""},
 "nic":{"code":"N13","detected":"x","key":"k"},
 "mem":{"code":"M63","detected":"x","key":"k"},
 "disk":{"code":"S10","detected":"x","key":"k"}},"config":""}`

const catcherUnmatchedGPU = `{"model_id":"I57N13M63S10","components":{
 "cpu":{"code":"I57","detected":"x","key":"k"},
 "gpu":{"code":"","detected":"NVIDIA B300 *8","key":"RAW:NVIDIA B300|8"},
 "nic":{"code":"N13","detected":"x","key":"k"},
 "mem":{"code":"M63","detected":"x","key":"k"},
 "disk":{"code":"S10","detected":"x","key":"k"}},"config":""}`

const catcherEmptyID = `{"model_id":"","components":{
 "cpu":{"code":"","detected":"","key":""},
 "gpu":{"code":"","detected":"","key":""},
 "nic":{"code":"","detected":"","key":""},
 "mem":{"code":"","detected":"","key":""},
 "disk":{"code":"","detected":"","key":""}},"config":""}`

func TestParseCatcherResult(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantID       string
		wantComplete bool
		wantMissing  []string
		wantErr      bool
	}{
		{name: "complete", input: catcherFull, wantID: "I57G90N13M63S10", wantComplete: true, wantMissing: []string{}},
		{name: "disk unmatched", input: catcherMissingDisk, wantID: "I57G90N13M63", wantComplete: false, wantMissing: []string{"disk"}},
		{name: "no gpu present", input: catcherNoGPU, wantID: "I57N13M63S10", wantComplete: true, wantMissing: []string{}},
		{name: "gpu present but unmatched", input: catcherUnmatchedGPU, wantID: "I57N13M63S10", wantComplete: false, wantMissing: []string{"gpu"}},
		{name: "empty model id", input: catcherEmptyID, wantID: "", wantComplete: false, wantMissing: []string{"cpu", "nic", "mem", "disk"}},
		{name: "corrupt json", input: `{"model_id":`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, sm, err := parseCatcherResult([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantComplete, sm.Complete)
			assert.Equal(t, tt.wantMissing, sm.Missing)
		})
	}
}

func TestParseCatcherResult_KeepsComponentsAndConfig(t *testing.T) {
	_, sm, err := parseCatcherResult([]byte(catcherFull))
	require.NoError(t, err)
	assert.Equal(t, ServerModelPart{Code: "I57", Detected: "INTEL(R) XEON(R) GOLD 6530 (2 socket)", Key: "Intel 6530|2"}, sm.Components["cpu"])
	assert.Len(t, sm.Components, 5)
	assert.Equal(t, "CPU: Intel 6530 *2\nGPU: 5090 *8", sm.Config)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./service -run TestParseCatcherResult -v`
Expected: 编译失败，`undefined: parseCatcherResult`。

- [ ] **Step 3: 实现**

`service/servermodel.go`：

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
package service

import (
	"encoding/json"
	"fmt"
	"time"
)

// ServerModelPart is one hardware part of the catcher result.
type ServerModelPart struct {
	Code     string `json:"code"`
	Detected string `json:"detected"`
	Key      string `json:"key"`
}

// ServerModel is the detail behind server_model_id, produced by the hardware
// team's catcher script. It is node metadata, not a health check.
type ServerModel struct {
	Complete       bool                       `json:"complete"`
	Missing        []string                   `json:"missing"`
	Components     map[string]ServerModelPart `json:"components,omitempty"`
	Config         string                     `json:"config,omitempty"`
	CatcherVersion string                     `json:"catcher_version,omitempty"`
	CollectedAt    time.Time                  `json:"collected_at"`
	Error          string                     `json:"error"`
}

// catcherResult mirrors result/catcher_result.json written by `catcher.bash json`.
type catcherResult struct {
	ModelID    string                     `json:"model_id"`
	Components map[string]ServerModelPart `json:"components"`
	Config     string                     `json:"config"`
}

// serverModelParts is the catcher's fixed ID order (CPU → GPU → NIC → MEM → DISK).
var serverModelParts = []string{"cpu", "gpu", "nic", "mem", "disk"}

// parseCatcherResult decodes a catcher result and derives Complete/Missing.
// A part is missing when it has no code; GPU is the exception — an empty GPU
// part with nothing detected is a GPU-less node, which catcher itself does not
// count as incomplete.
func parseCatcherResult(data []byte) (string, *ServerModel, error) {
	var r catcherResult
	if err := json.Unmarshal(data, &r); err != nil {
		return "", nil, fmt.Errorf("parse catcher result: %w", err)
	}
	missing := []string{}
	for _, name := range serverModelParts {
		p := r.Components[name]
		if p.Code != "" {
			continue
		}
		if name == "gpu" && p.Detected == "" && p.Key == "" {
			continue
		}
		missing = append(missing, name)
	}
	return r.ModelID, &ServerModel{
		Complete:   r.ModelID != "" && len(missing) == 0,
		Missing:    missing,
		Components: r.Components,
		Config:     r.Config,
	}, nil
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./service -run TestParseCatcherResult -v`
Expected: PASS（7 个子测试 + 1 个测试）。

- [ ] **Step 5: Commit**

```bash
git add service/servermodel.go service/servermodel_test.go
git commit -m "feat(servermodel): parse catcher result and derive completeness"
```

---

### Task 2: 执行 catcher（临时目录 + 进程组超时）

**Files:**
- Modify: `service/servermodel.go`
- Modify: `service/servermodel_test.go`

**Interfaces:**
- Consumes: `parseCatcherResult`（Task 1）
- Produces:
  - `func runCatcher(ctx context.Context, scriptPath string, timeout time.Duration) ([]byte, error)` —— 返回 `result/catcher_result.json` 原始字节。
  - `func readCatcherVersion(scriptPath string) string`

- [ ] **Step 1: 写失败测试**

追加到 `service/servermodel_test.go`（import 增加 `context`、`os`、`path/filepath`、`time`）：

```go
// writeFakeCatcher creates a fake catcher dir with the given script body and
// returns the path to catcher.bash. A lib/ file is included to verify the whole
// directory (not just the entry script) is copied.
func writeFakeCatcher(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "lib"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lib", "data.json"), []byte(catcherFull), 0644))
	script := filepath.Join(dir, "catcher.bash")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/bash\n"+body), 0755))
	return script
}

func TestRunCatcher_Success(t *testing.T) {
	// Mirrors real catcher: writes into its own dir and only prints a path.
	script := writeFakeCatcher(t, `D="$(cd "$(dirname "$0")" && pwd)"
[ "$1" = json ] || exit 9
mkdir -p "$D/result" && cp "$D/lib/data.json" "$D/result/catcher_result.json"
echo "JSON written: $D/result/catcher_result.json"`)
	srcDir := filepath.Dir(script)

	data, err := runCatcher(context.Background(), script, 10*time.Second)
	require.NoError(t, err)
	id, _, err := parseCatcherResult(data)
	require.NoError(t, err)
	assert.Equal(t, "I57G90N13M63S10", id)

	// The install dir must stay untouched (it may be read-only in production).
	_, statErr := os.Stat(filepath.Join(srcDir, "result"))
	assert.True(t, os.IsNotExist(statErr), "catcher must run in a temp copy, not the install dir")
}

func TestRunCatcher_NoResultFile(t *testing.T) {
	script := writeFakeCatcher(t, `exit 0`)
	_, err := runCatcher(context.Background(), script, 10*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catcher_result.json")
}

func TestRunCatcher_NonZeroExit(t *testing.T) {
	script := writeFakeCatcher(t, `echo boom >&2; exit 3`)
	_, err := runCatcher(context.Background(), script, 10*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestRunCatcher_MissingScript(t *testing.T) {
	_, err := runCatcher(context.Background(), filepath.Join(t.TempDir(), "nope", "catcher.bash"), 10*time.Second)
	require.Error(t, err)
}

func TestRunCatcher_TimeoutKillsProcessGroup(t *testing.T) {
	// A backgrounded grandchild keeps stdout open; without a process-group
	// kill, waiting for output would block until it exits (60s).
	script := writeFakeCatcher(t, `sleep 60 &
sleep 60`)
	start := time.Now()
	_, err := runCatcher(context.Background(), script, 500*time.Millisecond)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestReadCatcherVersion(t *testing.T) {
	script := writeFakeCatcher(t, "exit 0")
	assert.Equal(t, "", readCatcherVersion(script))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(script), "VERSION"), []byte("sha256:abc 2026-11-13\n"), 0644))
	assert.Equal(t, "sha256:abc 2026-11-13", readCatcherVersion(script))
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./service -run 'TestRunCatcher|TestReadCatcherVersion' -v`
Expected: 编译失败，`undefined: runCatcher`。

- [ ] **Step 3: 实现**

`service/servermodel.go` import 改为：

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)
```

追加：

```go
// runCatcher copies the catcher directory to a temp dir (catcher writes
// result/ next to itself and the install dir may be read-only), runs
// `catcher.bash json` in its own process group, and returns the result file.
// On timeout the whole group is killed: catcher spawns nvidia-smi/dmidecode,
// and a plain context kill only reaches bash, leaving grandchildren holding
// the output pipe open.
func runCatcher(ctx context.Context, scriptPath string, timeout time.Duration) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "sichek-catcher-")
	if err != nil {
		return nil, fmt.Errorf("mkdir temp: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := copyDir(filepath.Dir(scriptPath), tmp); err != nil {
		return nil, fmt.Errorf("copy catcher: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(tmp, filepath.Base(scriptPath)), "json")
	cmd.Dir = tmp
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("catcher timed out after %s", timeout)
		}
		return nil, fmt.Errorf("catcher failed: %w: %s", err, strings.TrimSpace(tail(out.String(), 512)))
	}
	data, err := os.ReadFile(filepath.Join(tmp, "result", "catcher_result.json"))
	if err != nil {
		return nil, fmt.Errorf("read catcher_result.json: %w", err)
	}
	return data, nil
}

// readCatcherVersion returns the VERSION file next to the script, or "".
func readCatcherVersion(scriptPath string) string {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(scriptPath), "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// copyDir copies regular files and directories from src into dst, skipping
// any stale result/ directory.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() && rel == "result" {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0755)
	})
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
```

说明：`cmd.Cancel` 与 `cmd.WaitDelay` 是 Go 1.20+ API，go.mod 为 1.23 可用。`WaitDelay` 兜底：即使有逃出进程组的孙进程握着管道，5s 后 `Run` 也会返回。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./service -run 'TestRunCatcher|TestReadCatcherVersion|TestParseCatcherResult' -v`
Expected: 全部 PASS，`TestRunCatcher_TimeoutKillsProcessGroup` 用时 < 2s。

- [ ] **Step 5: Commit**

```bash
git add service/servermodel.go service/servermodel_test.go
git commit -m "feat(servermodel): run catcher in temp copy with process-group timeout"
```

---

### Task 3: Snapshot 字段、配置加载与定时 runner

**Files:**
- Modify: `service/snapshot.go`（`Snapshot` struct 与新增方法）
- Modify: `service/snapshot_test.go`
- Modify: `service/servermodel.go`
- Modify: `service/servermodel_test.go`

**Interfaces:**
- Consumes: `parseCatcherResult`、`runCatcher`、`readCatcherVersion`、`ServerModel`
- Produces:
  - `Snapshot.ServerModelID string`（json `server_model_id,omitempty`）、`Snapshot.ServerModel *ServerModel`（json `server_model,omitempty`）
  - `func (s *SnapshotManager) SetServerModel(id string, sm *ServerModel)`
  - `type ServerModelConfig struct { Enable bool; ScriptPath string; InitialDelay, Interval, Timeout time.Duration }`
  - `func defaultServerModelConfig() ServerModelConfig`
  - `func LoadServerModelConfig(cfgFile string) (ServerModelConfig, error)`
  - `type ServerModelRunner struct`、`func NewServerModelRunner(cfg ServerModelConfig, snap *SnapshotManager, onUpdate func(id string, complete bool)) *ServerModelRunner`、`func (r *ServerModelRunner) RunOnce(ctx context.Context)`、`func (r *ServerModelRunner) Run(ctx context.Context)`

- [ ] **Step 1: 写失败测试（snapshot）**

追加到 `service/snapshot_test.go`：

```go
func TestSnapshotManager_SetServerModel(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.json")
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("snapshot:\n  enable: true\n  path: "+snapshotPath), 0644))

	mgr, err := NewSnapshotManager(cfgFile)
	require.NoError(t, err)
	mgr.SetServerModel("I57G90N13M63S10", &ServerModel{Complete: true, Missing: []string{}})

	data, err := os.ReadFile(snapshotPath)
	require.NoError(t, err)
	var snapshot Snapshot
	require.NoError(t, json.Unmarshal(data, &snapshot))
	assert.Equal(t, "I57G90N13M63S10", snapshot.ServerModelID)
	require.NotNil(t, snapshot.ServerModel)
	assert.True(t, snapshot.ServerModel.Complete)
	assert.Contains(t, string(data), `"server_model_id"`)
}

func TestSnapshotManager_ServerModelOmittedBeforeFirstRun(t *testing.T) {
	tmpDir := t.TempDir()
	snapshotPath := filepath.Join(tmpDir, "snapshot.json")
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte("snapshot:\n  enable: true\n  path: "+snapshotPath), 0644))

	mgr, err := NewSnapshotManager(cfgFile)
	require.NoError(t, err)
	mgr.SetIssues(nil)

	data, err := os.ReadFile(snapshotPath)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "server_model")
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./service -run 'TestSnapshotManager_.*ServerModel' -v`
Expected: 编译失败，`snapshot.ServerModelID undefined`。

- [ ] **Step 3: 实现 snapshot 部分**

`service/snapshot.go` 的 `Snapshot` struct，在 `Issues` 字段之后追加：

```go
	// ServerModelID is the hardware team's 机型ID (catcher), refreshed hourly
	// by ServerModelRunner. Omitted until the first catcher run.
	ServerModelID string       `json:"server_model_id,omitempty"`
	ServerModel   *ServerModel `json:"server_model,omitempty"`
```

在 `SetIssues` 方法之后追加：

```go
// SetServerModel records the node's server model ID and detail, then persists.
func (s *SnapshotManager) SetServerModel(id string, sm *ServerModel) {
	if !s.enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Timestamp = time.Now()
	s.data.ServerModelID = id
	s.data.ServerModel = sm

	if err := s.persist(); err != nil {
		logrus.WithField("service", "snapshot").Errorf("Failed to persist snapshot: %v", err)
	}
}
```

Run: `go test ./service -run 'TestSnapshotManager' -v` → Expected: PASS。

- [ ] **Step 4: 写失败测试（配置 + runner）**

追加到 `service/servermodel_test.go`（import 增加 `encoding/json`）：

```go
func TestLoadServerModelConfig(t *testing.T) {
	def := defaultServerModelConfig()
	tests := []struct {
		name string
		yaml string
		want ServerModelConfig
	}{
		{name: "no block", yaml: "snapshot:\n  enable: true\n", want: def},
		{name: "partial block", yaml: "server_model:\n  interval: 30m\n", want: func() ServerModelConfig {
			c := def
			c.Interval = 30 * time.Minute
			return c
		}()},
		{name: "zero durations fall back", yaml: "server_model:\n  enable: true\n  interval: 0s\n  timeout: 0s\n  initial_delay: 0s\n", want: def},
		{name: "disabled", yaml: "server_model:\n  enable: false\n", want: func() ServerModelConfig {
			c := def
			c.Enable = false
			return c
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "cfg.yaml")
			require.NoError(t, os.WriteFile(f, []byte(tt.yaml), 0644))
			got, err := LoadServerModelConfig(f)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultServerModelConfig(t *testing.T) {
	c := defaultServerModelConfig()
	assert.True(t, c.Enable)
	assert.Equal(t, "/var/sichek/scripts/catcher/catcher.bash", c.ScriptPath)
	assert.Equal(t, 5*time.Minute, c.InitialDelay)
	assert.Equal(t, time.Hour, c.Interval)
	assert.Equal(t, 120*time.Second, c.Timeout)
}

// newTestSnapshot returns an enabled SnapshotManager writing to a temp file.
func newTestSnapshot(t *testing.T) (*SnapshotManager, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")
	cfg := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte("snapshot:\n  enable: true\n  path: "+path), 0644))
	mgr, err := NewSnapshotManager(cfg)
	require.NoError(t, err)
	return mgr, path
}

func readSnapshot(t *testing.T, path string) Snapshot {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var s Snapshot
	require.NoError(t, json.Unmarshal(data, &s))
	return s
}

const fakeCatcherOK = `D="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$D/result" && cp "$D/lib/data.json" "$D/result/catcher_result.json"`

func TestServerModelRunner_SuccessThenFailureKeepsLastGood(t *testing.T) {
	script := writeFakeCatcher(t, fakeCatcherOK)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(script), "VERSION"), []byte("sha256:abc 2026-11-13\n"), 0644))
	snap, path := newTestSnapshot(t)
	cfg := defaultServerModelConfig()
	cfg.ScriptPath = script
	cfg.Timeout = 10 * time.Second

	var gotID string
	var gotComplete bool
	calls := 0
	r := NewServerModelRunner(cfg, snap, func(id string, complete bool) { gotID, gotComplete, calls = id, complete, calls+1 })

	r.RunOnce(context.Background())
	s := readSnapshot(t, path)
	assert.Equal(t, "I57G90N13M63S10", s.ServerModelID)
	require.NotNil(t, s.ServerModel)
	assert.True(t, s.ServerModel.Complete)
	assert.Equal(t, "sha256:abc 2026-11-13", s.ServerModel.CatcherVersion)
	assert.Empty(t, s.ServerModel.Error)
	assert.False(t, s.ServerModel.CollectedAt.IsZero())
	assert.Equal(t, "I57G90N13M63S10", gotID)
	assert.True(t, gotComplete)
	firstAt := s.ServerModel.CollectedAt

	// Break the script: next run must keep the last good ID and only set error.
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/bash\nexit 1\n"), 0755))
	time.Sleep(10 * time.Millisecond)
	r.RunOnce(context.Background())
	s = readSnapshot(t, path)
	assert.Equal(t, "I57G90N13M63S10", s.ServerModelID)
	require.NotNil(t, s.ServerModel)
	assert.True(t, s.ServerModel.Complete)
	assert.NotEmpty(t, s.ServerModel.Error)
	assert.True(t, s.ServerModel.CollectedAt.After(firstAt))
	assert.Equal(t, 1, calls, "metrics callback must not fire on failure")
}

func TestServerModelRunner_FirstRunFailure(t *testing.T) {
	snap, path := newTestSnapshot(t)
	cfg := defaultServerModelConfig()
	cfg.ScriptPath = filepath.Join(t.TempDir(), "missing", "catcher.bash")
	calls := 0
	r := NewServerModelRunner(cfg, snap, func(string, bool) { calls++ })

	r.RunOnce(context.Background())
	s := readSnapshot(t, path)
	assert.Empty(t, s.ServerModelID)
	require.NotNil(t, s.ServerModel)
	assert.NotEmpty(t, s.ServerModel.Error)
	assert.False(t, s.ServerModel.Complete)
	assert.Equal(t, 0, calls)
}

func TestServerModelRunner_RunHonorsInitialDelayAndStops(t *testing.T) {
	script := writeFakeCatcher(t, fakeCatcherOK)
	snap, path := newTestSnapshot(t)
	cfg := defaultServerModelConfig()
	cfg.ScriptPath = script
	cfg.InitialDelay = 200 * time.Millisecond
	cfg.Interval = time.Hour
	r := NewServerModelRunner(cfg, snap, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "must not run before initial delay")
	require.Eventually(t, func() bool {
		_, err := os.Stat(path)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)
	assert.Equal(t, "I57G90N13M63S10", readSnapshot(t, path).ServerModelID)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}
```

- [ ] **Step 5: 运行确认失败**

Run: `go test ./service -run 'TestLoadServerModelConfig|TestDefaultServerModelConfig|TestServerModelRunner' -v`
Expected: 编译失败，`undefined: defaultServerModelConfig`。

- [ ] **Step 6: 实现配置 + runner**

`service/servermodel.go` import 再加 `"sync"`、`"github.com/sirupsen/logrus"`、`"gopkg.in/yaml.v3"`。追加：

```go
// ServerModelConfig controls the periodic catcher run.
type ServerModelConfig struct {
	Enable       bool          `json:"enable"        yaml:"enable"`
	ScriptPath   string        `json:"script_path"   yaml:"script_path"`
	InitialDelay time.Duration `json:"initial_delay" yaml:"initial_delay"`
	Interval     time.Duration `json:"interval"      yaml:"interval"`
	Timeout      time.Duration `json:"timeout"       yaml:"timeout"`
}

type serverModelFile struct {
	ServerModel ServerModelConfig `json:"server_model" yaml:"server_model"`
}

// defaultServerModelConfig: first run 5m after start (clear of the startup
// NVML burst), then hourly.
func defaultServerModelConfig() ServerModelConfig {
	return ServerModelConfig{
		Enable:       true,
		ScriptPath:   "/var/sichek/scripts/catcher/catcher.bash",
		InitialDelay: 5 * time.Minute,
		Interval:     time.Hour,
		Timeout:      120 * time.Second,
	}
}

// LoadServerModelConfig parses the server_model block from cfgFile, filling
// zero-valued fields with defaults. Missing file or block yields defaults.
func LoadServerModelConfig(cfgFile string) (ServerModelConfig, error) {
	def := defaultServerModelConfig()
	if cfgFile == "" {
		return def, nil
	}
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		if os.IsNotExist(err) {
			return def, nil
		}
		return def, fmt.Errorf("load server_model config: %w", err)
	}
	f := serverModelFile{ServerModel: def}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return def, fmt.Errorf("load server_model config: %w", err)
	}
	out := f.ServerModel
	if out.ScriptPath == "" {
		out.ScriptPath = def.ScriptPath
	}
	if out.InitialDelay <= 0 {
		out.InitialDelay = def.InitialDelay
	}
	if out.Interval <= 0 {
		out.Interval = def.Interval
	}
	if out.Timeout <= 0 {
		out.Timeout = def.Timeout
	}
	return out, nil
}

// ServerModelRunner periodically runs catcher and publishes the result to the
// snapshot. On failure it keeps the last good ID/detail and only updates Error.
type ServerModelRunner struct {
	cfg      ServerModelConfig
	snap     *SnapshotManager
	onUpdate func(id string, complete bool)

	mu     sync.Mutex
	lastID string
	last   *ServerModel
}

// NewServerModelRunner builds a runner. onUpdate (may be nil) fires after each
// successful run, e.g. to refresh the Prometheus info metric.
func NewServerModelRunner(cfg ServerModelConfig, snap *SnapshotManager, onUpdate func(id string, complete bool)) *ServerModelRunner {
	return &ServerModelRunner{cfg: cfg, snap: snap, onUpdate: onUpdate}
}

// Run waits InitialDelay, then runs catcher every Interval until ctx is done.
func (r *ServerModelRunner) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(r.cfg.InitialDelay):
	}
	r.RunOnce(ctx)
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.RunOnce(ctx)
		}
	}
}

// RunOnce executes catcher a single time and publishes the outcome.
func (r *ServerModelRunner) RunOnce(ctx context.Context) {
	log := logrus.WithField("service", "server_model")
	now := time.Now()
	id, sm, err := r.collect(ctx)

	r.mu.Lock()
	if err != nil {
		log.Warnf("catcher run failed: %v", err)
		sm = &ServerModel{Missing: []string{}}
		if r.last != nil {
			cp := *r.last
			sm = &cp
		}
		id = r.lastID
		sm.Error = err.Error()
	} else {
		sm.CatcherVersion = readCatcherVersion(r.cfg.ScriptPath)
		r.lastID, r.last = id, sm
		log.Infof("server_model_id=%q complete=%v missing=%v", id, sm.Complete, sm.Missing)
	}
	sm.CollectedAt = now
	r.mu.Unlock()

	if r.snap != nil {
		r.snap.SetServerModel(id, sm)
	}
	if err == nil && r.onUpdate != nil {
		r.onUpdate(id, sm.Complete)
	}
}

func (r *ServerModelRunner) collect(ctx context.Context) (string, *ServerModel, error) {
	data, err := runCatcher(ctx, r.cfg.ScriptPath, r.cfg.Timeout)
	if err != nil {
		return "", nil, err
	}
	return parseCatcherResult(data)
}
```

注意：失败分支拷贝 `*r.last` 再写 `Error`，`r.last` 自身保持 `Error==""`，下次成功会整体替换；`Components` map 与 `Missing` 切片与 r.last 共享但此后不再被修改，安全。

- [ ] **Step 7: 运行确认通过**

Run: `go test ./service -v -run 'ServerModel|Catcher|Snapshot'`
Expected: 全部 PASS。

再跑整个包确认无回归：`go test ./service/...` → PASS。

- [ ] **Step 8: Commit**

```bash
git add service/snapshot.go service/snapshot_test.go service/servermodel.go service/servermodel_test.go
git commit -m "feat(servermodel): hourly catcher runner publishing server_model_id to snapshot"
```

---

### Task 4: Prometheus 指标

**Files:**
- Create: `metrics/servermodel.go`
- Create: `metrics/servermodel_test.go`

**Interfaces:**
- Produces: `func SetServerModelInfo(id string, complete bool)` —— 导出 `sichek_server_model_info{server_model_id, complete, node} 1`，替换旧 label 组合。

- [ ] **Step 1: 写失败测试**

`metrics/servermodel_test.go`（带版权头）：

```go
package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serverModelSeries(t *testing.T) []*dto.Metric {
	t.Helper()
	ch := make(chan prometheus.Metric, 10)
	serverModelInfo.Collect(ch)
	close(ch)
	var out []*dto.Metric
	for m := range ch {
		var d dto.Metric
		require.NoError(t, m.Write(&d))
		out = append(out, &d)
	}
	return out
}

func labelValue(m *dto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}

func TestSetServerModelInfo_ReplacesPreviousSeries(t *testing.T) {
	SetServerModelInfo("I57G90N13M63", false)
	SetServerModelInfo("I57G90N13M63S10", true)

	series := serverModelSeries(t)
	require.Len(t, series, 1)
	assert.Equal(t, "I57G90N13M63S10", labelValue(series[0], "server_model_id"))
	assert.Equal(t, "true", labelValue(series[0], "complete"))
	assert.NotEmpty(t, labelValue(series[0], "node"))
	assert.Equal(t, 1.0, series[0].GetGauge().GetValue())
}
```

先确认 `dto` 包已 vendored：`ls vendor/github.com/prometheus/client_model/go` 应存在（client_golang 依赖它）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./metrics -run TestSetServerModelInfo -v`
Expected: 编译失败，`undefined: SetServerModelInfo`。

- [ ] **Step 3: 实现**

`metrics/servermodel.go`（带版权头）：

```go
package metrics

import (
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// serverModelInfo exposes the node's server_model_id (catcher 机型ID) as an
// info-style gauge. Only one series per node exists at a time.
var serverModelInfo = func() *prometheus.GaugeVec {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: MetricPrefix + "_server_model_info",
		Help: "Server model ID detected by catcher; value is always 1.",
	}, []string{"server_model_id", "complete", "node"})
	prometheus.MustRegister(g)
	return g
}()

// SetServerModelInfo replaces the node's server model series.
func SetServerModelInfo(id string, complete bool) {
	node, err := os.Hostname()
	if err != nil {
		node = "unknown"
	}
	serverModelInfo.Reset()
	serverModelInfo.WithLabelValues(id, strconv.FormatBool(complete), node).Set(1)
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./metrics -v -run TestSetServerModelInfo` → PASS；再 `go test ./metrics/...` → PASS（确认包级 MustRegister 无重名冲突）。

- [ ] **Step 5: Commit**

```bash
git add metrics/servermodel.go metrics/servermodel_test.go
git commit -m "feat(metrics): add sichek_server_model_info gauge"
```

---

### Task 5: 接入 daemon、默认配置与镜像依赖

**Files:**
- Modify: `service/daemon.go`（`DaemonService` struct、`NewService`、`Run`）
- Modify: `config/default_user_config.yaml`
- Modify: `docker/Dockerfile`、`docker/Dockerfile.cuda128`、`docker/Dockerfile.cuda130`、`docker/Dockerfile.centos8`

**Interfaces:**
- Consumes: `LoadServerModelConfig`、`NewServerModelRunner`、`(*ServerModelRunner).Run`（Task 3）；`metrics.SetServerModelInfo`（Task 4）

- [ ] **Step 1: daemon 接线**

`service/daemon.go`：

`DaemonService` struct 在 `reporter *Reporter` 后加：

```go
	serverModel          *ServerModelRunner
```

`NewService` 中 reporter 构造块之后加：

```go
	// Server model: hourly catcher run publishing server_model_id (node metadata).
	var serverModel *ServerModelRunner
	smCfg, err := LoadServerModelConfig(cfgFile)
	if err != nil {
		logrus.WithField("daemon", "new").Warnf("load server_model config failed: %v", err)
	}
	if smCfg.Enable {
		serverModel = NewServerModelRunner(smCfg, snapshotMgr, metrics.SetServerModelInfo)
	}
```

注意：这里的 `err` 赋值会被 `NewService` 的命名返回值 `err` 捕获（defer 里 `if err != nil { cancel() }`）。为避免配置加载失败误触发 cancel，用局部变量：把 `smCfg, err :=` 写成 `smCfg, smErr :=`，相应 `if smErr != nil`。

`daemonService := &DaemonService{...}` 字面量中加 `serverModel: serverModel,`。

`Run()` 中 `if d.reporter != nil { go d.reporter.Run(d.ctx) }` 之后加：

```go
	if d.serverModel != nil {
		go d.serverModel.Run(d.ctx)
	}
```

- [ ] **Step 2: 编译与全量测试**

Run: `go build ./... && go vet ./service ./metrics && go test ./service/... ./metrics/...`
Expected: 无错误，全部 PASS。

- [ ] **Step 3: 默认配置**

`config/default_user_config.yaml` 在 `reporter:` 块之后（`nvidia:` 之前）插入：

```yaml
server_model:
  enable: true   # run hardware team's catcher to report server_model_id
  script_path: "/var/sichek/scripts/catcher/catcher.bash"
  initial_delay: 5m   # first run after daemon start (avoid startup NVML burst)
  interval: 1h
  timeout: 120s
```

用一次性测试验证仓库里的默认配置文件解析结果等于代码默认值：

```bash
cd /root/devnet/sichek && cat > service/zz_cfgcheck_test.go <<'EOF'
package service
import "testing"
func TestZZDefaultUserConfigServerModel(t *testing.T) {
	c, err := LoadServerModelConfig("../config/default_user_config.yaml")
	if err != nil || c != defaultServerModelConfig() { t.Fatalf("%+v %v", c, err) }
}
EOF
go test ./service -run TestZZDefaultUserConfigServerModel -v; rm service/zz_cfgcheck_test.go
```
Expected: PASS（之后已删除临时文件）。

- [ ] **Step 4: 镜像依赖**

`docker/Dockerfile`、`docker/Dockerfile.cuda128`、`docker/Dockerfile.cuda130` 的**运行时阶段**（第二个 `apt-get install`，含 `libnuma1 numactl rdmacm-utils perftest xz-utils` 那行）改为：

```dockerfile
        libnuma1 numactl rdmacm-utils perftest xz-utils \
        dmidecode ethtool && \
```

`docker/Dockerfile.centos8` 运行时阶段（含 `numactl numactl-libs rdma-core perftest xz` 那行）改为：

```dockerfile
        numactl numactl-libs rdma-core perftest xz \
        dmidecode ethtool && \
```

用 `grep -n "dmidecode" docker/Dockerfile*` 确认四个文件各命中一次，且位于 `FROM` 第二阶段之后。

- [ ] **Step 5: 构建二进制**

Run: `make`
Expected: 产出 `build/bin/sichek`，无报错。

- [ ] **Step 6: Commit**

```bash
git add service/daemon.go config/default_user_config.yaml docker/Dockerfile docker/Dockerfile.cuda128 docker/Dockerfile.cuda130 docker/Dockerfile.centos8
git commit -m "feat(daemon): wire server_model runner; add catcher deps to images"
```

---

### Task 6: 现场回归

使用 `sichek-field-regression` skill。要点（来自既往经验）：

- 测试 daemon 必须隔离：config 复制到 `/tmp/sichek-test/`，`reporter.enable=false`（或指向测试 endpoint），`snapshot.path` 改到 `/tmp/sichek-test/snapshot.json`，`SICHEK_CONFIG_DIR=/tmp/sichek-test/config`。
- 为避免等 5 分钟，测试 config 设 `server_model.initial_delay: 10s`、`interval: 2m`；`script_path` 指向 `/tmp/sichek-test/catcher/catcher.bash`（同步上传 `scripts/catcher/`）。
- 节点：一台 B300（如 10.220.77.49）+ 一台 H 系列（如 clnet36）。

- [ ] **Step 1:** 在节点上先手跑 `sudo bash /tmp/sichek-test/catcher/catcher.bash detect` 记录人工结果（ID、未匹配部件）。
- [ ] **Step 2:** 起测试 daemon，等待 > initial_delay + 单次运行时间，检查 `jq '.server_model_id, .server_model' /tmp/sichek-test/snapshot.json` 与人工结果一致（ID、complete、missing）。
- [ ] **Step 3:** `curl -s localhost:<测试端口>/metrics | grep sichek_server_model_info`，确认恰好一条且 label 正确。
- [ ] **Step 4:** 把 `script_path` 改成不存在的路径重启 → snapshot 有 `server_model.error`，无 `server_model_id`；daemon 其他组件正常。
- [ ] **Step 5:** 运行期间 `ps -ef | grep -E 'catcher|dmidecode|nvidia-smi'` 确认运行结束后无残留进程；`ls /tmp | grep sichek-catcher-` 无残留临时目录。
- [ ] **Step 6:** 输出回归报告（markdown），列出两台节点的 ID、完整性、发现的未匹配部件（如 B300 GPU 可能不在 catcher 命名表——这是 catalog 问题，反馈硬件团队，不在 sichek 修）。清理测试 daemon。
