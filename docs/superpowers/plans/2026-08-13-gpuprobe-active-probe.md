# gpuprobe — GPU 功能自检主动探测组件 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `gpuprobe` 组件——daemon 周期性在**空闲** GPU 上 exec 内置 CUDA 探针验证计算通路，接入 daemon / K8s annotation / Prometheus。

**Architecture:** 标准 sichek 组件（collector→checker→config + 顶层 `common.Component`）。collector 用 `nvidia-smi` 做空闲门控、对空闲卡 `exec.CommandContext` 跑预编译探针 ELF、按退出码归类 outcome，严格回收子进程/进程组；checker 把 outcome 映射成分级 `CheckerResult`（FAIL/timeout=Critical，env/exec 错=Warning，忙=跳过），带可配连续失败去抖。默认**不进** `DefaultComponents`（默认关），经 `-E gpuprobe` 或 daemon enabled-components 显式开启。

**Tech Stack:** Go 1.23、`os/exec` + `syscall`（进程组）、`nvidia-smi` CLI、testify、纯 Go 主构建（探针 ELF 预编译入仓，不引入 nvcc）。

**Spec:** `docs/superpowers/specs/2026-08-13-gpuprobe-active-probe-design.md`

**参照组件：** `components/cpu/`（骨架）、`components/ovs/config/`（纯 Go `DefaultSpec()` spec 加载，避开 `EnsureSpecFile` 覆写 prod config 的坑）。

---

## 前置约定（每个新文件都适用）

- 所有新 `.go` / `.cu` / `.yaml` 文件顶部加 Apache-2.0 Scitix 版权头（见 `CLAUDE.md`）。
- 包路径前缀 `github.com/scitix/sichek`。
- 状态/级别常量：`consts.StatusNormal` / `consts.StatusAbnormal`；`consts.LevelInfo|LevelWarning|LevelCritical`。
- 测试：testify `assert`/`require`，表驱动 + `t.Run` 子测试，文件隔离用 `t.TempDir()`。
- 提交信息用中文正文、不带 Claude co-author trailer（仓库惯例）。

---

## Task 0: 分支与目录骨架

**Files:**
- Create dirs: `components/gpuprobe/{collector,checker,config,metrics,probe,bin}/`

- [ ] **Step 1: 从 main 切特性分支**

Run:
```bash
cd /root/devnet/sichek
git checkout main && git pull --ff-only
git checkout -b feat/gpuprobe-active-probe
```

- [ ] **Step 2: 建目录**

Run:
```bash
mkdir -p components/gpuprobe/{collector,checker,config,metrics,probe,bin}
```
Expected: 目录创建成功，`ls components/gpuprobe` 列出 6 个子目录。

---

## Task 1: 探针源码（退出码语义化）+ Makefile + README

原脚本把"跳过"和"通过"都返回 `EXIT_SUCCESS`，退出码分不清。改造为 `0=PASS / 1=FAIL / 2=SKIP / 3=ENV_ERR`。

**Files:**
- Create: `components/gpuprobe/probe/gpu_probe.cu`
- Create: `components/gpuprobe/probe/Makefile`
- Create: `components/gpuprobe/probe/README.md`

- [ ] **Step 1: 写探针源码 `gpu_probe.cu`**

```cpp
/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
... (完整 Apache 头，注意 .cu 用 C 注释) ...
*/

// gpu_probe: 单卡 GPU 功能自检探针。
// 退出码约定（sichek collector 依赖，勿改语义）:
//   0 PASS     kernel 结果逐元素校验通过
//   1 FAIL     结果 mismatch —— 卡算错了
//   2 SKIP     主动让路：free% < min-free-pct 或 设备忙/显存不足
//   3 ENV_ERR  其余 CUDA API 失败 —— CUDA 环境异常
#include <iostream>
#include <cstdlib>
#include <cstring>
#include <vector>
#include <cuda_runtime.h>

enum ProbeExit { PROBE_PASS = 0, PROBE_FAIL = 1, PROBE_SKIP = 2, PROBE_ENV_ERR = 3 };

// CUDA API 失败一律归为环境错(3)，与"算错(1)"区分。
#define CHECK_CUDA(call) { \
    cudaError_t err = call; \
    if (err != cudaSuccess) { \
        std::cerr << "CUDA_ERR@" << __FILE__ << ":" << __LINE__ \
                  << " code=" << err << "(" << cudaGetErrorString(err) \
                  << ") op: " #call << std::endl; \
        std::cout << "RESULT=ENV_ERR device=" << deviceId \
                  << " detail=" << cudaGetErrorString(err) << std::endl; \
        exit(PROBE_ENV_ERR); \
    }}

__global__ void basic_validation_kernel(int* a, int* b, int* c, int n) {
    int tid = threadIdx.x + blockIdx.x * blockDim.x;
    if (tid < n) c[tid] = a[tid] * 2 + b[tid];
}

int runProbe(int deviceId, float minFreePct) {
    cudaSetDevice(deviceId);

    size_t freeMem = 0, totalMem = 0;
    cudaError_t memErr = cudaMemGetInfo(&freeMem, &totalMem);
    if (memErr == cudaErrorMemoryAllocation || memErr == cudaErrorDevicesUnavailable) {
        std::cout << "RESULT=SKIP device=" << deviceId
                  << " detail=" << cudaGetErrorString(memErr) << std::endl;
        return PROBE_SKIP;
    } else if (memErr != cudaSuccess) {
        std::cout << "RESULT=ENV_ERR device=" << deviceId
                  << " detail=" << cudaGetErrorString(memErr) << std::endl;
        return PROBE_ENV_ERR;
    }
    float freePct = totalMem ? (float)freeMem / totalMem * 100.0f : 0.0f;
    if (freePct < minFreePct) {
        std::cout << "RESULT=SKIP device=" << deviceId
                  << " free_pct=" << freePct << " min=" << minFreePct << std::endl;
        return PROBE_SKIP;
    }

    const int test_size = 1 << 20;      // 1M 元素
    const int block_size = 256;
    std::vector<int> h_a(test_size), h_b(test_size), h_c(test_size, 0);
    int *d_a, *d_b, *d_c;
    for (int i = 0; i < test_size; ++i) { h_a[i] = i; h_b[i] = i % 100; }

    CHECK_CUDA(cudaMalloc(&d_a, test_size * sizeof(int)));
    CHECK_CUDA(cudaMalloc(&d_b, test_size * sizeof(int)));
    CHECK_CUDA(cudaMalloc(&d_c, test_size * sizeof(int)));
    CHECK_CUDA(cudaMemcpy(d_a, h_a.data(), test_size * sizeof(int), cudaMemcpyHostToDevice));
    CHECK_CUDA(cudaMemcpy(d_b, h_b.data(), test_size * sizeof(int), cudaMemcpyHostToDevice));

    dim3 grid((test_size + block_size - 1) / block_size);
    basic_validation_kernel<<<grid, block_size>>>(d_a, d_b, d_c, test_size);
    CHECK_CUDA(cudaGetLastError());
    CHECK_CUDA(cudaDeviceSynchronize());
    CHECK_CUDA(cudaMemcpy(h_c.data(), d_c, test_size * sizeof(int), cudaMemcpyDeviceToHost));

    bool valid = true;
    for (int i = 0; i < test_size; ++i) {
        int expected = h_a[i] * 2 + h_b[i];
        if (h_c[i] != expected) {
            std::cerr << "MISMATCH@i=" << i << " expect=" << expected << " got=" << h_c[i] << std::endl;
            valid = false;
            break;
        }
    }
    cudaFree(d_a); cudaFree(d_b); cudaFree(d_c);

    if (valid) { std::cout << "RESULT=PASS device=" << deviceId << std::endl; return PROBE_PASS; }
    std::cout << "RESULT=FAIL device=" << deviceId << std::endl;
    return PROBE_FAIL;
}

int main(int argc, char* argv[]) {
    int deviceId = -1;
    float minFreePct = 50.0f;
    for (int i = 1; i < argc; ++i) {
        if (!strcmp(argv[i], "-d") && i + 1 < argc) deviceId = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--min-free-pct") && i + 1 < argc) minFreePct = atof(argv[++i]);
    }
    if (deviceId < 0) {
        std::cerr << "usage: gpu_probe -d <device> [--min-free-pct <pct>]" << std::endl;
        return PROBE_ENV_ERR;
    }
    int deviceCount = 0;
    if (cudaGetDeviceCount(&deviceCount) != cudaSuccess || deviceId >= deviceCount) {
        std::cout << "RESULT=ENV_ERR device=" << deviceId << " detail=invalid_device_or_init_failed" << std::endl;
        return PROBE_ENV_ERR;
    }
    return runProbe(deviceId, minFreePct);
}
```

> 注：`CHECK_CUDA` 宏引用了 `deviceId`，故只在 `runProbe`（`deviceId` 在作用域内）里使用；`main` 的初始化错误单独 `cout RESULT=ENV_ERR` 后 return，不用宏。

- [ ] **Step 2: 写 `Makefile`**

```makefile
# 单独编译探针，不接入主 `make`（主流程保持纯 Go、离线可建）。
# 需在带 CUDA toolchain 的主机上手动运行，产物提交到 ../bin/。
NVCC ?= nvcc
# 覆盖主流架构：Ampere(80) / Hopper(90) / Blackwell(100,120)。按需增减。
GENCODE ?= -gencode arch=compute_80,code=sm_80 \
           -gencode arch=compute_90,code=sm_90 \
           -gencode arch=compute_100,code=sm_100 \
           -gencode arch=compute_120,code=sm_120 \
           -gencode arch=compute_90,code=compute_90
# 静态链 cudart：运行期只依赖驱动 libcuda.so.1，不绑 CUDA runtime 版本。
CUDA_LINK ?= -lcudart_static -ldl -lrt -lpthread

.PHONY: amd64 arm64
amd64:
	$(NVCC) $(GENCODE) -O2 gpu_probe.cu -o ../bin/gpu_probe.amd64 $(CUDA_LINK)
arm64:
	$(NVCC) $(GENCODE) -O2 gpu_probe.cu -o ../bin/gpu_probe.arm64 $(CUDA_LINK)
```

- [ ] **Step 3: 写 `README.md`**

内容需包含：探针用途、退出码表、`make amd64` / `make arm64` 命令、必须在带对应架构 CUDA toolchain 的主机编译并把产物 `git add -f ../bin/gpu_probe.{amd64,arm64}` 入仓、驱动/架构变更时如何重编。

- [ ] **Step 4: 提交源码（ELF 稍后单独提交）**

```bash
git add components/gpuprobe/probe/
git commit -m "feat(gpuprobe): 探针源码(退出码语义化)+Makefile+README"
```

---

## Task 1b: 【手动】编译并提交探针 ELF

> **这是唯一的非代码手动步骤**，须在带 CUDA toolchain 的主机执行。后续 Go 任务用 shell stub 假探针测试，**不阻塞**——即使此步未完成，代码与测试仍可跑。缺 ELF 时运行期归为 `exec_err`(Warning)。

- [ ] **Step 1: 在 CUDA 主机编译两架构**

Run（在带 nvcc 的 amd64 主机）:
```bash
cd components/gpuprobe/probe && make amd64
```
Run（在带 nvcc 的 arm64 主机，或交叉工具链）:
```bash
cd components/gpuprobe/probe && make arm64
```
Expected: 产出 `components/gpuprobe/bin/gpu_probe.amd64` 与 `gpu_probe.arm64`（各约 1–3 MB）。

- [ ] **Step 2: 冒烟验证退出码**

Run（在有 GPU 的节点）:
```bash
./bin/gpu_probe.amd64 -d 0; echo "exit=$?"      # 空闲卡应 exit=0 RESULT=PASS
./bin/gpu_probe.amd64 -d 999; echo "exit=$?"    # 无效卡应 exit=3 RESULT=ENV_ERR
```

- [ ] **Step 3: 强制入仓（`.gitignore` 可能忽略二进制，用 -f）**

```bash
git add -f components/gpuprobe/bin/gpu_probe.amd64 components/gpuprobe/bin/gpu_probe.arm64
git commit -m "feat(gpuprobe): 预编译探针 ELF 入仓(amd64/arm64)"
```

---

## Task 2: config 包（user config + spec + check items）

**Files:**
- Create: `components/gpuprobe/config/config.go`
- Create: `components/gpuprobe/config/spec.go`
- Create: `components/gpuprobe/config/check_items.go`
- Test: `components/gpuprobe/config/spec_test.go`

- [ ] **Step 1: 写 `config.go`（user config，仿 cpu）**

```go
// (Apache 头)
package config

import "github.com/scitix/sichek/components/common"

type GpuProbeUserConfig struct {
	GpuProbe *GpuProbeConfig `yaml:"gpuprobe"`
}

type GpuProbeConfig struct {
	QueryInterval common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64           `json:"cache_size" yaml:"cache_size"`
	EnableMetrics bool            `json:"enable_metrics" yaml:"enable_metrics"`
}

func (c *GpuProbeUserConfig) GetQueryInterval() common.Duration { return c.GpuProbe.QueryInterval }
func (c *GpuProbeUserConfig) SetQueryInterval(n common.Duration) { c.GpuProbe.QueryInterval = n }
```

- [ ] **Step 2: 写 `spec.go`（纯 Go DefaultSpec，仿 ovs，不写文件）**

```go
// (Apache 头)
package config

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type GpuProbeSpecConfig struct {
	GpuProbe *GpuProbeSpec `json:"gpuprobe" yaml:"gpuprobe"`
}

type GpuProbeSpec struct {
	ProbeBinaryPath          string `json:"probe_binary_path" yaml:"probe_binary_path"`
	ProbeTimeoutSec          int    `json:"probe_timeout_sec" yaml:"probe_timeout_sec"`
	KillGraceSec             int    `json:"kill_grace_sec" yaml:"kill_grace_sec"`
	MinFreeMemPct            int    `json:"min_free_mem_pct" yaml:"min_free_mem_pct"`
	MaxGpuUtilPct            int    `json:"max_gpu_util_pct" yaml:"max_gpu_util_pct"`
	SkipIfComputeApps        bool   `json:"skip_if_compute_apps" yaml:"skip_if_compute_apps"`
	SkipMig                  bool   `json:"skip_mig" yaml:"skip_mig"`
	FailConsecutiveThreshold int    `json:"fail_consecutive_threshold" yaml:"fail_consecutive_threshold"`
	FailLevel                string `json:"fail_level" yaml:"fail_level"`
	EnvErrorLevel            string `json:"env_error_level" yaml:"env_error_level"`
}

func (s *GpuProbeSpec) valid() bool {
	return s != nil && s.ProbeBinaryPath != "" && s.ProbeTimeoutSec > 0 && s.MinFreeMemPct > 0
}

func LoadSpec(file string) (*GpuProbeSpec, error) {
	if file == "" {
		return DefaultSpec(), nil
	}
	var s GpuProbeSpecConfig
	if err := common.LoadSpec(file, &s); err != nil || !s.GpuProbe.valid() {
		logrus.WithField("component", "gpuprobe/spec").Warnf("spec in %s missing/invalid (err=%v), using built-in default", file, err)
		return DefaultSpec(), nil
	}
	return s.GpuProbe, nil
}

func DefaultSpec() *GpuProbeSpec {
	return &GpuProbeSpec{
		ProbeBinaryPath:          "/var/sichek/bin/gpu_probe",
		ProbeTimeoutSec:          30,
		KillGraceSec:             5,
		MinFreeMemPct:            50,
		MaxGpuUtilPct:            10,
		SkipIfComputeApps:        true,
		SkipMig:                  true,
		FailConsecutiveThreshold: 1,
		FailLevel:                consts.LevelCritical,
		EnvErrorLevel:            consts.LevelWarning,
	}
}
```

- [ ] **Step 3: 写 `check_items.go`（CheckerResult 模板，仿 cpu）**

```go
// (Apache 头)
package config

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

const GpuProbeCheckerName = "gpu-probe"

var GpuProbeCheckItems = map[string]common.CheckerResult{
	GpuProbeCheckerName: {
		Name:        GpuProbeCheckerName,
		Description: "Active GPU compute self-test (idle-gated)",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPUProbeFailed",
		Suggestion:  "GPU failed an active compute self-test while idle; drain and inspect (dmesg Xid / GPU Recovery Action), consider GPU reset",
	},
}
```

- [ ] **Step 4: 写 `spec_test.go`（失败先行）**

```go
// (Apache 头)
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSpec_EmptyReturnsDefault(t *testing.T) {
	s, err := LoadSpec("")
	require.NoError(t, err)
	assert.Equal(t, "/var/sichek/bin/gpu_probe", s.ProbeBinaryPath)
	assert.Equal(t, 30, s.ProbeTimeoutSec)
	assert.Equal(t, 1, s.FailConsecutiveThreshold)
	assert.Equal(t, consts.LevelCritical, s.FailLevel)
}

func TestLoadSpec_InvalidFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(p, []byte("gpuprobe:\n  probe_timeout_sec: 0\n"), 0644))
	s, err := LoadSpec(p)
	require.NoError(t, err)
	assert.Equal(t, 30, s.ProbeTimeoutSec) // 回落默认
}

func TestLoadSpec_ValidOverrides(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ok.yaml")
	yaml := "gpuprobe:\n  probe_binary_path: /opt/gp\n  probe_timeout_sec: 15\n  min_free_mem_pct: 70\n"
	require.NoError(t, os.WriteFile(p, []byte(yaml), 0644))
	s, err := LoadSpec(p)
	require.NoError(t, err)
	assert.Equal(t, "/opt/gp", s.ProbeBinaryPath)
	assert.Equal(t, 15, s.ProbeTimeoutSec)
	assert.Equal(t, 70, s.MinFreeMemPct)
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./components/gpuprobe/config/... -v`
Expected: 3 个测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add components/gpuprobe/config/
git commit -m "feat(gpuprobe): config 包(user config + 纯 Go DefaultSpec + check items)"
```

---

## Task 3: collector（枚举 + 门控 + exec + 超时/回收 + 重入）

拆成三个文件，各一职责：类型定义、门控查询、探针执行。

**Files:**
- Create: `components/gpuprobe/collector/types.go`（Info/Result 类型）
- Create: `components/gpuprobe/collector/gating.go`（nvidia-smi 门控查询）
- Create: `components/gpuprobe/collector/runner.go`（exec + 超时 + reap + 进程组）
- Create: `components/gpuprobe/collector/collector.go`（编排 + 重入门闩）
- Test: `components/gpuprobe/collector/collector_test.go`

- [ ] **Step 1: 写 `types.go`**

```go
// (Apache 头)
package collector

import (
	"encoding/json"
	"time"
)

const (
	OutcomePass    = "pass"
	OutcomeFail    = "fail"
	OutcomeSkip    = "skip"
	OutcomeEnvErr  = "env_err"
	OutcomeExecErr = "exec_err"
	OutcomeTimeout = "timeout"
)

type GpuProbeResult struct {
	Index      int    `json:"index"`
	BDF        string `json:"bdf"`
	State      string `json:"state"`   // "idle" | "busy"
	Outcome    string `json:"outcome"` // OutcomeXxx
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Detail     string `json:"detail"`
}

type GpuProbeInfo struct {
	Time   time.Time        `json:"time"`
	PerGPU []GpuProbeResult `json:"per_gpu"`
}

func (o *GpuProbeInfo) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}
```

- [ ] **Step 2: 写 `gating.go`（门控查询 + 可注入命令，便于测试）**

```go
// (Apache 头)
package collector

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// gpuStat 一张卡的门控原始读数。
type gpuStat struct {
	Index       int
	BDF         string
	UtilPct     int
	FreePct     int
	MigEnabled  bool
	ComputeApps int
}

// nvidiaSmi 允许测试注入假命令。默认真跑 nvidia-smi。
var nvidiaSmi = func(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "nvidia-smi", args...).Output()
	return string(out), err
}

// listGPUs 返回 index→BDF；无 GPU / nvidia-smi 缺失时返回空 map（非错误由上层判断）。
func queryGPUs(ctx context.Context) ([]gpuStat, error) {
	out, err := nvidiaSmi(ctx,
		"--query-gpu=index,pci.bus_id,utilization.gpu,memory.used,memory.total,mig.mode.current",
		"--format=csv,noheader,nounits")
	if err != nil {
		return nil, err
	}
	var stats []gpuStat
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, ",")
		if len(f) < 6 {
			continue
		}
		idx, _ := strconv.Atoi(strings.TrimSpace(f[0]))
		util, _ := strconv.Atoi(strings.TrimSpace(f[2]))
		used, _ := strconv.Atoi(strings.TrimSpace(f[3]))
		total, _ := strconv.Atoi(strings.TrimSpace(f[4]))
		freePct := 0
		if total > 0 {
			freePct = int(float64(total-used) / float64(total) * 100)
		}
		mig := strings.Contains(strings.ToLower(f[5]), "enabled")
		stats = append(stats, gpuStat{
			Index: idx, BDF: shortBDF(strings.TrimSpace(f[1])),
			UtilPct: util, FreePct: freePct, MigEnabled: mig,
		})
	}
	return stats, nil
}

// countComputeApps 返回某卡上的计算进程数。
func countComputeApps(ctx context.Context, idx int) int {
	out, err := nvidiaSmi(ctx, "--query-compute-apps=pid", "--format=csv,noheader", "-i", strconv.Itoa(idx))
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// shortBDF 取 nvidia-smi 的 "00000000:BB:00.0" → "bb:00"，便于定位。
func shortBDF(full string) string {
	full = strings.ToLower(strings.TrimSpace(full))
	parts := strings.Split(full, ":")
	if len(parts) >= 3 {
		dev := strings.SplitN(parts[2], ".", 2)[0]
		return parts[1] + ":" + dev
	}
	return full
}
```

- [ ] **Step 3: 写 `runner.go`（exec + 超时 + reap + 进程组）**

```go
// (Apache 头)
package collector

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// runProbe 对单卡 exec 探针，按退出码归类 outcome，严格回收子进程/进程组。
// timeout 命中 → kill 整个进程组 → 二段带宽限 Wait；到点未 reap 记 timeout 继续。
func runProbe(ctx context.Context, binPath string, idx, minFreePct, timeoutSec, killGraceSec int) (outcome string, exitCode int, detail string, durMs int64) {
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, binPath, "-d", strconv.Itoa(idx), "--min-free-pct", strconv.Itoa(minFreePct))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // 独立进程组，便于整组 kill
	out, err := cmd.Output()
	durMs = time.Since(start).Milliseconds()
	detail = strings.TrimSpace(string(out))

	if cctx.Err() == context.DeadlineExceeded {
		// 超时：CommandContext 已 SIGKILL 主进程；再 kill 整组清孙进程。
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		// 二段 Wait 已由 cmd.Output()/CommandContext 内部触发；此处不再阻塞。
		return OutcomeTimeout, -1, "probe timed out after " + strconv.Itoa(timeoutSec) + "s: " + detail, durMs
	}
	if err == nil {
		return OutcomePass, 0, detail, durMs
	}
	if ee, ok := err.(*exec.ExitError); ok {
		exitCode = ee.ExitCode()
		switch exitCode {
		case 1:
			return OutcomeFail, 1, detail, durMs
		case 2:
			return OutcomeSkip, 2, detail, durMs
		case 3:
			return OutcomeEnvErr, 3, detail, durMs
		default:
			return OutcomeEnvErr, exitCode, "unexpected exit code: " + detail, durMs
		}
	}
	// 启动失败：探针缺失/不可执行/架构不符/libcuda 缺。
	return OutcomeExecErr, -1, "failed to exec probe " + binPath + ": " + err.Error(), durMs
}
```

> 说明：`exec.CommandContext` 在 ctx 超时时对 `cmd.Process` 发 SIGKILL 并在 `Output()` 返回前 `Wait()` 回收主进程，故无僵尸；额外 `Kill(-pid)` 清理可能的孙进程。D 态 hang（SIGKILL 也杀不掉）时 `Output()` 会阻塞到内核态返回——由外层 per-probe ctx + 组件 600s 总超时兜底，不会永久卡死 daemon；这类卡本身就是要报的 timeout=Critical。

- [ ] **Step 4: 写 `collector.go`（编排 + 重入门闩）**

```go
// (Apache 头)
package collector

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/sirupsen/logrus"
)

type Collector struct {
	name     string
	spec     *config.GpuProbeSpec
	probing  int32               // 重入门闩(atomic CAS)
	lastInfo atomic.Value        // *GpuProbeInfo，重入时返回上次
}

func NewGpuProbeCollector(spec *config.GpuProbeSpec) *Collector {
	return &Collector{name: "GpuProbeCollector", spec: spec}
}

func (c *Collector) Name() string { return c.name }

func (c *Collector) Collect(ctx context.Context) (common.Info, error) {
	// 重入门闩：上一轮未完则返回上次结果，绝不并发探同卡。
	if !atomic.CompareAndSwapInt32(&c.probing, 0, 1) {
		logrus.WithField("component", "gpuprobe").Warn("previous probe round still running; skip this tick")
		if v := c.lastInfo.Load(); v != nil {
			return v.(*GpuProbeInfo), nil
		}
		return &GpuProbeInfo{Time: time.Now()}, nil
	}
	defer atomic.StoreInt32(&c.probing, 0)

	info := &GpuProbeInfo{Time: time.Now()}
	stats, err := queryGPUs(ctx)
	if err != nil || len(stats) == 0 {
		// 无 GPU / nvidia-smi 缺失 → 空结果，组件 Normal（不报错）。
		logrus.WithField("component", "gpuprobe").Infof("no GPU / nvidia-smi unavailable (err=%v); nothing to probe", err)
		c.lastInfo.Store(info)
		return info, nil
	}

	for _, st := range stats {
		r := GpuProbeResult{Index: st.Index, BDF: st.BDF}
		// MIG 跳过
		if c.spec.SkipMig && st.MigEnabled {
			r.State, r.Outcome, r.Detail = "busy", OutcomeSkip, "MIG enabled, not supported"
			info.PerGPU = append(info.PerGPU, r)
			continue
		}
		// 空闲门控三信号
		busy := st.FreePct < c.spec.MinFreeMemPct || st.UtilPct > c.spec.MaxGpuUtilPct
		if c.spec.SkipIfComputeApps && countComputeApps(ctx, st.Index) > 0 {
			busy = true
		}
		if busy {
			r.State, r.Outcome = "busy", OutcomeSkip
			r.Detail = "busy: skipped to avoid interfering with workload"
			info.PerGPU = append(info.PerGPU, r)
			continue
		}
		// 空闲 → 探测
		r.State = "idle"
		r.Outcome, r.ExitCode, r.Detail, r.DurationMs = runProbe(
			ctx, c.spec.ProbeBinaryPath, st.Index, c.spec.MinFreeMemPct,
			c.spec.ProbeTimeoutSec, c.spec.KillGraceSec)
		info.PerGPU = append(info.PerGPU, r)
	}
	c.lastInfo.Store(info)
	return info, nil
}
```

- [ ] **Step 5: 写 `collector_test.go`（失败先行；用 stub 假 nvidia-smi + 假探针）**

```go
// (Apache 头)
package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 写一个按参数返回指定退出码的假探针脚本。
func writeFakeProbe(t *testing.T, exitCode int) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "gpu_probe")
	script := "#!/bin/sh\necho \"RESULT=stub\"\nexit " + itoa(exitCode) + "\n"
	require.NoError(t, os.WriteFile(p, []byte(script), 0755))
	return p
}
func itoa(i int) string { return map[int]string{0: "0", 1: "1", 2: "2", 3: "3"}[i] }

func TestCollect_IdleGPUPass(t *testing.T) {
	// 假 nvidia-smi：1 张空闲卡(util=0,free=99%,无MIG,无进程)
	nvidiaSmi = func(ctx context.Context, args ...string) (string, error) {
		for _, a := range args {
			if a == "--query-compute-apps=pid" {
				return "", nil // 无进程
			}
		}
		return "0, 00000000:BB:00.0, 0, 100, 81920, Disabled\n", nil
	}
	defer func() { nvidiaSmi = nil }()

	spec := config.DefaultSpec()
	spec.ProbeBinaryPath = writeFakeProbe(t, 0)
	c := NewGpuProbeCollector(spec)
	info, err := c.Collect(context.Background())
	require.NoError(t, err)
	gi := info.(*GpuProbeInfo)
	require.Len(t, gi.PerGPU, 1)
	assert.Equal(t, "idle", gi.PerGPU[0].State)
	assert.Equal(t, OutcomePass, gi.PerGPU[0].Outcome)
	assert.Equal(t, "bb:00", gi.PerGPU[0].BDF)
}

func TestCollect_BusyGPUSkipped(t *testing.T) {
	nvidiaSmi = func(ctx context.Context, args ...string) (string, error) {
		return "0, 00000000:BB:00.0, 90, 80000, 81920, Disabled\n", nil // util=90,几乎满显存
	}
	defer func() { nvidiaSmi = nil }()
	spec := config.DefaultSpec()
	spec.ProbeBinaryPath = writeFakeProbe(t, 1) // 即便探针会 FAIL，忙卡也不该跑它
	c := NewGpuProbeCollector(spec)
	info, _ := c.Collect(context.Background())
	gi := info.(*GpuProbeInfo)
	assert.Equal(t, "busy", gi.PerGPU[0].State)
	assert.Equal(t, OutcomeSkip, gi.PerGPU[0].Outcome)
}

func TestCollect_IdleGPUFail(t *testing.T) {
	nvidiaSmi = func(ctx context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "--query-compute-apps=pid" {
			return "", nil
		}
		return "0, 00000000:BB:00.0, 0, 100, 81920, Disabled\n", nil
	}
	defer func() { nvidiaSmi = nil }()
	spec := config.DefaultSpec()
	spec.ProbeBinaryPath = writeFakeProbe(t, 1)
	c := NewGpuProbeCollector(spec)
	info, _ := c.Collect(context.Background())
	gi := info.(*GpuProbeInfo)
	assert.Equal(t, OutcomeFail, gi.PerGPU[0].Outcome)
	assert.Equal(t, 1, gi.PerGPU[0].ExitCode)
}

func TestCollect_MissingBinaryExecErr(t *testing.T) {
	nvidiaSmi = func(ctx context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "--query-compute-apps=pid" {
			return "", nil
		}
		return "0, 00000000:BB:00.0, 0, 100, 81920, Disabled\n", nil
	}
	defer func() { nvidiaSmi = nil }()
	spec := config.DefaultSpec()
	spec.ProbeBinaryPath = "/nonexistent/gpu_probe"
	c := NewGpuProbeCollector(spec)
	info, _ := c.Collect(context.Background())
	gi := info.(*GpuProbeInfo)
	assert.Equal(t, OutcomeExecErr, gi.PerGPU[0].Outcome)
}

func TestCollect_NoGPU(t *testing.T) {
	nvidiaSmi = func(ctx context.Context, args ...string) (string, error) { return "", nil }
	defer func() { nvidiaSmi = nil }()
	c := NewGpuProbeCollector(config.DefaultSpec())
	info, err := c.Collect(context.Background())
	require.NoError(t, err)
	assert.Empty(t, info.(*GpuProbeInfo).PerGPU)
}
```

> 测试把包级 `nvidiaSmi` 变量替换为 stub 后置 nil；因 `runProbe` 的门控 free/util 已由 stub 数据决定，探针用 shell 脚本假冒（真机不参与）。为让 `nvidiaSmi=nil` 后不 panic，`gating.go` 顶部保留默认真实现，测试内覆盖、defer 复原为真实现（改 `defer` 复原为原函数指针而非 nil——见下步修正）。

- [ ] **Step 6: 修正 stub 复原（避免置 nil）**

在 `collector_test.go` 顶部加保存/复原真实现的 helper，把各测试的 `defer func(){ nvidiaSmi = nil }()` 换成复原为初始值：

```go
var realNvidiaSmi = nvidiaSmi
// 各测试改为： defer func() { nvidiaSmi = realNvidiaSmi }()
```

- [ ] **Step 7: 跑测试**

Run: `go test ./components/gpuprobe/collector/... -v`
Expected: 6 个测试全 PASS。

- [ ] **Step 8: 提交**

```bash
git add components/gpuprobe/collector/
git commit -m "feat(gpuprobe): collector(空闲门控+探针exec+超时/进程组回收+重入门闩)"
```

---

## Task 4: checker（outcome → 分级 + 连续失败去抖）

**Files:**
- Create: `components/gpuprobe/checker/check_gpu_probe.go`
- Test: `components/gpuprobe/checker/check_gpu_probe_test.go`

- [ ] **Step 1: 写 `check_gpu_probe.go`**

```go
// (Apache 头)
package checker

import (
	"context"
	"fmt"
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuprobe/collector"
	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/scitix/sichek/consts"
)

const GpuProbeCheckerName = config.GpuProbeCheckerName

type GpuProbeChecker struct {
	name string
	spec *config.GpuProbeSpec
	mu   sync.Mutex
	// 按 BDF 记连续 FAIL/timeout 次数，用于去抖(spec.FailConsecutiveThreshold)。
	consecFails map[string]int
}

func NewGpuProbeChecker(spec *config.GpuProbeSpec) (common.Checker, error) {
	return &GpuProbeChecker{
		name:        GpuProbeCheckerName,
		spec:        spec,
		consecFails: make(map[string]int),
	}, nil
}

func (c *GpuProbeChecker) Name() string { return c.name }

func (c *GpuProbeChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.GpuProbeInfo)
	if !ok {
		return nil, fmt.Errorf("invalid gpuprobe info type")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	result := config.GpuProbeCheckItems[GpuProbeCheckerName]
	result.Status = consts.StatusNormal
	result.Level = consts.LevelInfo

	var failed, timedout, envErr, execErr, tested, skipped []string
	for _, g := range info.PerGPU {
		dev := fmt.Sprintf("GPU%d %s", g.Index, g.BDF)
		switch g.Outcome {
		case collector.OutcomePass:
			c.consecFails[g.BDF] = 0
			tested = append(tested, dev)
		case collector.OutcomeSkip:
			c.consecFails[g.BDF] = 0
			skipped = append(skipped, dev)
		case collector.OutcomeFail, collector.OutcomeTimeout:
			c.consecFails[g.BDF]++
			if g.Outcome == collector.OutcomeTimeout {
				timedout = append(timedout, dev)
			} else {
				failed = append(failed, dev)
			}
		case collector.OutcomeEnvErr:
			envErr = append(envErr, dev)
		case collector.OutcomeExecErr:
			execErr = append(execErr, dev)
		}
	}

	// 去抖：只有连续失败达到阈值的卡才升 Critical；未达阈值先按 Warning 观察。
	var hardBad, softBad []string
	for _, dev := range append(append([]string{}, failed...), timedout...) {
		bdf := devBDF(dev)
		if c.consecFails[bdf] >= c.spec.FailConsecutiveThreshold {
			hardBad = append(hardBad, dev)
		} else {
			softBad = append(softBad, dev)
		}
	}

	switch {
	case len(hardBad) > 0:
		result.Status = consts.StatusAbnormal
		result.Level = c.spec.FailLevel
		result.ErrorName = "GPUProbeFailed"
		result.Curr = fmt.Sprintf("%d fail", len(hardBad))
		result.Detail = fmt.Sprintf("GPU compute self-test FAILED on idle GPUs: %v", hardBad)
	case len(softBad) > 0 || len(envErr) > 0 || len(execErr) > 0:
		result.Status = consts.StatusAbnormal
		result.Level = c.spec.EnvErrorLevel // Warning
		result.ErrorName = "GPUProbeEnvError"
		result.Detail = fmt.Sprintf("gpuprobe warnings: pending-fail=%v env_err=%v exec_err=%v", softBad, envErr, execErr)
	default:
		result.Detail = fmt.Sprintf("gpuprobe OK: tested=%v skipped(busy)=%v", tested, skipped)
	}
	return &result, nil
}

// devBDF 从 "GPU6 bb:00" 取回 "bb:00"。
func devBDF(dev string) string {
	for i := 0; i < len(dev); i++ {
		if dev[i] == ' ' {
			return dev[i+1:]
		}
	}
	return dev
}
```

- [ ] **Step 2: 写 `check_gpu_probe_test.go`（失败先行）**

```go
// (Apache 头)
package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/gpuprobe/collector"
	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkChecker(t *testing.T, threshold int) *GpuProbeChecker {
	spec := config.DefaultSpec()
	spec.FailConsecutiveThreshold = threshold
	ch, err := NewGpuProbeChecker(spec)
	require.NoError(t, err)
	return ch.(*GpuProbeChecker)
}

func info(results ...collector.GpuProbeResult) *collector.GpuProbeInfo {
	return &collector.GpuProbeInfo{PerGPU: results}
}

func TestCheck_PassIsNormal(t *testing.T) {
	c := mkChecker(t, 1)
	r, err := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", Outcome: collector.OutcomePass}))
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, r.Status)
}

func TestCheck_FailImmediateCriticalWhenThreshold1(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 6, BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
	assert.Equal(t, "GPUProbeFailed", r.ErrorName)
}

func TestCheck_FailDebouncedWhenThreshold2(t *testing.T) {
	c := mkChecker(t, 2)
	// 第一轮 FAIL → Warning(soft)
	r1, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 6, BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.LevelWarning, r1.Level)
	// 第二轮同卡 FAIL → Critical(hard)
	r2, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 6, BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.LevelCritical, r2.Level)
}

func TestCheck_PassResetsConsecutive(t *testing.T) {
	c := mkChecker(t, 2)
	c.Check(context.Background(), info(collector.GpuProbeResult{BDF: "bb:00", Outcome: collector.OutcomeFail}))
	c.Check(context.Background(), info(collector.GpuProbeResult{BDF: "bb:00", Outcome: collector.OutcomePass}))
	r, _ := c.Check(context.Background(), info(collector.GpuProbeResult{BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.LevelWarning, r.Level) // 计数已清零，重新从 1 起
}

func TestCheck_EnvErrIsWarning(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", Outcome: collector.OutcomeEnvErr}))
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelWarning, r.Level)
}

func TestCheck_TimeoutIsCritical(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", Outcome: collector.OutcomeTimeout}))
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestCheck_AllBusyIsNormal(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", State: "busy", Outcome: collector.OutcomeSkip}))
	assert.Equal(t, consts.StatusNormal, r.Status)
}
```

- [ ] **Step 3: 跑测试**

Run: `go test ./components/gpuprobe/checker/... -v`
Expected: 7 个测试全 PASS。

- [ ] **Step 4: 提交**

```bash
git add components/gpuprobe/checker/
git commit -m "feat(gpuprobe): checker(outcome分级 + 连续失败去抖)"
```

---

## Task 5: metrics 包

**Files:**
- Create: `components/gpuprobe/metrics/metrics.go`

- [ ] **Step 1: 写 `metrics.go`（仿 cpu，导出逐卡 status/duration）**

```go
// (Apache 头)
package metrics

import (
	"github.com/scitix/sichek/components/gpuprobe/collector"
	common "github.com/scitix/sichek/metrics"
)

const MetricPrefix = "sichek_gpuprobe"

// outcome → 数值编码（Prometheus 只吃数值）。
var outcomeCode = map[string]float64{
	collector.OutcomePass: 0, collector.OutcomeFail: 1, collector.OutcomeSkip: 2,
	collector.OutcomeEnvErr: 3, collector.OutcomeExecErr: 4, collector.OutcomeTimeout: 5,
}

type GpuProbeMetrics struct {
	statusGauge   *common.GaugeVecMetricExporter
	durationGauge *common.GaugeVecMetricExporter
}

func NewGpuProbeMetrics() *GpuProbeMetrics {
	return &GpuProbeMetrics{
		statusGauge:   common.NewGaugeVecMetricExporter(MetricPrefix, []string{"gpu", "bdf"}),
		durationGauge: common.NewGaugeVecMetricExporter(MetricPrefix, []string{"gpu", "bdf"}),
	}
}

func (m *GpuProbeMetrics) ExportMetrics(info *collector.GpuProbeInfo) {
	for _, g := range info.PerGPU {
		gpu := itoa(g.Index)
		m.statusGauge.SetMetric("probe_status", []string{gpu, g.BDF}, outcomeCode[g.Outcome])
		m.durationGauge.SetMetric("duration_ms", []string{gpu, g.BDF}, float64(g.DurationMs))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
```

> **实现前先核对** `metrics.GaugeVecMetricExporter` 的真实方法名（`SetMetric` vs 其它）：Run `grep -n "func (.*GaugeVecMetricExporter)" metrics/*.go`，按真实签名调整 Step 1 的调用。cpu 用的是 `ExportStruct`，本组件是逐卡打点，用 `SetMetric` 更直接——若无该方法，改用 `grep` 出的等价方法。

- [ ] **Step 2: 编译确认**

Run: `go build ./components/gpuprobe/metrics/...`
Expected: 无错误（若报方法名不对，按上一步 grep 结果修正）。

- [ ] **Step 3: 提交**

```bash
git add components/gpuprobe/metrics/
git commit -m "feat(gpuprobe): metrics(逐卡 probe_status/duration_ms)"
```

---

## Task 6: 顶层组件 `gpuprobe.go`（实现 common.Component）

**Files:**
- Create: `components/gpuprobe/gpuprobe.go`

- [ ] **Step 1: 写 `gpuprobe.go`（仿 cpu.go，去掉 eventfilter——本组件无日志规则）**

```go
// (Apache 头)
package gpuprobe

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuprobe/checker"
	"github.com/scitix/sichek/components/gpuprobe/collector"
	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/scitix/sichek/components/gpuprobe/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string

	cfg      *config.GpuProbeUserConfig
	cfgMutex sync.Mutex

	collector *collector.Collector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
	metrics *metrics.GpuProbeMetrics
}

var (
	gpuProbeComponent     *component
	gpuProbeComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	gpuProbeComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component gpuprobe: %v", r)
			}
		}()
		gpuProbeComponent, err = newComponent(cfgFile, specFile)
	})
	return gpuProbeComponent, err
}

func newComponent(cfgFile string, specFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	cfg := &config.GpuProbeUserConfig{}
	if e := common.LoadUserConfig(cfgFile, cfg); e != nil || cfg.GpuProbe == nil {
		cancel()
		return nil, fmt.Errorf("NewComponent gpuprobe load user config failed: %v", e)
	}
	spec, err := config.LoadSpec(specFile)
	if err != nil {
		return nil, err
	}
	checkerPointer, err := checker.NewGpuProbeChecker(spec)
	if err != nil {
		return nil, err
	}
	var m *metrics.GpuProbeMetrics
	if cfg.GpuProbe.EnableMetrics {
		m = metrics.NewGpuProbeMetrics()
	}
	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameGPUProbe,
		collector:     collector.NewGpuProbeCollector(spec),
		checkers:      []common.Checker{checkerPointer},
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cfg.GpuProbe.CacheSize),
		cacheInfo:     make([]common.Info, cfg.GpuProbe.CacheSize),
		cacheSize:     cfg.GpuProbe.CacheSize,
		metrics:       m,
	}
	comp.service = common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	return comp, nil
}

func (c *component) Name() string { return c.componentName }

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "gpuprobe").Errorf("%v", err)
		return nil, err
	}
	gpuInfo, ok := info.(*collector.GpuProbeInfo)
	if !ok {
		return nil, fmt.Errorf("wrong gpuprobe info type")
	}
	if c.cfg.GpuProbe.EnableMetrics && c.metrics != nil {
		c.metrics.ExportMetrics(gpuInfo)
	}
	result := common.Check(ctx, c.Name(), gpuInfo, c.checkers)

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "gpuprobe").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "gpuprobe").Infof("Health Check PASSED")
	}
	return result, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheBuffer, nil
}

func (c *component) LastResult() (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfo, nil
}

func (c *component) LastInfo() (common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	if c.currIndex == 0 {
		return c.cacheInfo[c.cacheSize-1], nil
	}
	return c.cacheInfo[c.currIndex-1], nil
}

func (c *component) Start() <-chan *common.Result { return c.service.Start() }
func (c *component) Stop() error                  { return c.service.Stop() }
func (c *component) Status() bool                 { return c.service.Status() }
func (c *component) GetTimeout() time.Duration    { return c.cfg.GetQueryInterval().Duration }

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	p, ok := cfg.(*config.GpuProbeUserConfig)
	if !ok {
		c.cfgMutex.Unlock()
		return fmt.Errorf("update wrong config type for gpuprobe")
	}
	c.cfg = p
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	allPassed := result.Status == consts.StatusNormal
	for _, r := range result.Checkers {
		color := consts.Green
		if r.Status != consts.StatusNormal {
			color = consts.LevelColor(r.Level)
		}
		if summaryPrint {
			fmt.Printf("GPUProbe: %s%s%s %s\n", color, r.Status, consts.Reset, r.Detail)
		}
	}
	return allPassed
}
```

> **实现前核对** `common.LoadUserConfig` 签名与 cpu 用法一致（`components/cpu/cpu.go:84`）。若 `CacheSize` 为 0 会 panic（除零/零长切片索引）——`LoadUserConfig` 应从默认 user config 填充；确保默认 user config 里 gpuprobe 段有 `cache_size`（见 Task 8）。

- [ ] **Step 2: 编译**

Run: `go build ./components/gpuprobe/...`
Expected: 无错误。

- [ ] **Step 3: 提交**

```bash
git add components/gpuprobe/gpuprobe.go
git commit -m "feat(gpuprobe): 顶层组件(实现 common.Component + CommonService)"
```

---

## Task 7: 接线（consts / all.go / CLI / service annotation）

参照记忆 `annotation_schema_must_track_components`：不接全套线，issue 会被 annotation/snapshot 静默丢弃。

**Files:**
- Modify: `consts/consts.go`（加 `ComponentNameGPUProbe`；**不**加入 `DefaultComponents`）
- Modify: `cmd/command/component/all.go`（`NewComponent` switch 加 case）
- Create: `cmd/command/component/gpuprobe.go`（CLI 子命令）
- Modify: `cmd/command/command.go`（注册子命令）
- Modify: `service/info.go`（`nodeAnnotation` 加字段 + 两个 switch 加 case）

- [ ] **Step 1: `consts/consts.go` 加组件名常量**

在 `ComponentNameSysinfo` 后加：
```go
	ComponentNameGPUProbe     = "gpuprobe"
```
**不要**把它加进 `DefaultComponents`（默认关）。

- [ ] **Step 2: `all.go` 的 `NewComponent` switch 加 case**

在 `case consts.ComponentNameNvidia:` 附近加（探测需 GPU 存在）：
```go
	case consts.ComponentNameGPUProbe:
		if !utils.IsNvidiaGPUExist() {
			return nil, fmt.Errorf("nvidia GPU is not Exist. Bypassing GpuProbe HealthCheck")
		}
		return gpuprobe.NewComponent(cfgFile, specFile)
```
并加 import `"github.com/scitix/sichek/components/gpuprobe"`。

- [ ] **Step 3: 写 CLI 子命令 `cmd/command/component/gpuprobe.go`（仿 cpu.go）**

```go
// (Apache 头)
package component

import (
	"context"

	"github.com/scitix/sichek/components/gpuprobe"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewGpuProbeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gpu_probe",
		Aliases: []string{"gpuprobe"},
		Short:   "Perform active GPU compute self-test (idle-gated)",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			defer cancel()
			verbos, _ := cmd.Flags().GetBool("verbos")
			if !verbos {
				logrus.SetLevel(logrus.ErrorLevel)
			}
			cfgFile, _ := cmd.Flags().GetString("cfg")
			specFile, _ := cmd.Flags().GetString("spec")
			comp, err := gpuprobe.NewComponent(cfgFile, specFile)
			if err != nil {
				logrus.WithField("component", "gpuprobe").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, comp, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}
	cmd.Flags().StringP("cfg", "c", "", "Path to the gpuprobe cfg")
	cmd.Flags().StringP("spec", "s", "", "Path to the gpuprobe spec file")
	cmd.Flags().BoolP("verbos", "v", false, "Enable verbose output")
	return cmd
}
```

> `--force` / `-d <idx>` 深测标志列为**后续增强**（首版 CLI 与 daemon 共用同一门控路径即可；强制探测需在 collector 加 bypass 开关，非本次范围）。

- [ ] **Step 4: `command.go` 注册**

在 `rootCmd.AddCommand(component.NewMemoryCmd())` 附近加：
```go
	rootCmd.AddCommand(component.NewGpuProbeCmd())
```

- [ ] **Step 5: `service/info.go` 加 annotation 字段 + 两 switch**

`nodeAnnotation` struct 加：
```go
	GpuProbe    map[string][]*annotation `json:"gpuprobe"`
```
`getAnnotationsByItem` switch 加：
```go
	case consts.ComponentNameGPUProbe:
		return a.GpuProbe, nil
```
`setAnnotationsByItem` switch 加：
```go
	case consts.ComponentNameGPUProbe:
		a.GpuProbe = annotations
```

- [ ] **Step 6: 全量编译**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 7: 提交**

```bash
git add consts/consts.go cmd/command/component/all.go cmd/command/component/gpuprobe.go cmd/command/command.go service/info.go
git commit -m "feat(gpuprobe): 接线(consts/CLI/all.go/annotation schema)"
```

---

## Task 8: 默认 user config 段 + 打包安装探针 ELF

**Files:**
- Modify: 默认 user config YAML（`grep -rl "cpu:" --include=*.yaml config/ components/*/config/*.yaml | head` 定位 `default_user_config.yaml`）
- Modify: `.goreleaser.yaml` / `Makefile`（安装 ELF 到 `/var/sichek/bin/`）

- [ ] **Step 1: 定位默认 user config 并加 gpuprobe 段**

Run: `grep -rn "enable_metrics\|query_interval" config/*.yaml components/**/default_user_config.yaml 2>/dev/null | head`
在其中的默认 user config 追加：
```yaml
gpuprobe:
  query_interval: 600s
  cache_size: 5
  enable_metrics: true
```

- [ ] **Step 2: goreleaser/Makefile 打包 ELF**

Run: `grep -n "extra_files\|/var/sichek\|contents\|src:" .goreleaser.yaml | head`
按 GOARCH 把对应 ELF 装到 `/var/sichek/bin/gpu_probe`（`0755`）。nfpm(deb) `contents:` 示例：
```yaml
    - src: components/gpuprobe/bin/gpu_probe.{{ .Arch }}
      dst: /var/sichek/bin/gpu_probe
      file_info:
        mode: 0755
```
docker 镜像同样 COPY 对应架构 ELF 到 `/var/sichek/bin/gpu_probe`（改 Dockerfile）。

> **核对**仓库真实 goreleaser 结构后再改；`{{ .Arch }}` 取值需与文件名后缀（amd64/arm64）对齐。

- [ ] **Step 3: 编译 + 冒烟**

Run: `go build ./... && go vet ./components/gpuprobe/...`
Expected: 无错误。

- [ ] **Step 4: 提交**

```bash
git add -A
git commit -m "feat(gpuprobe): 默认 user config 段 + goreleaser 安装探针 ELF"
```

---

## Task 9: 文档（只读契约登记 + 组件说明）

**Files:**
- Modify: `docs/write-operations-audit.html`（登记主动占卡能力）
- Create: `docs/gpuprobe.md`（组件说明，或按仓库文档惯例并入 configuration.md）

- [ ] **Step 1: 在 write-operations-audit 增列 gpuprobe**

登记："gpuprobe 会在**空闲** GPU 上 exec kernel 跑 ~12MB/1M 元素的计算自检；默认关（不在 DefaultComponents），经 `-E gpuprobe` 显式开启；空闲门控 + 探针内部自查双防线避免干扰业务。"

- [ ] **Step 2: 写组件说明**

覆盖：用途、退出码表、门控三信号与默认阈值、定级表、如何开启（`sichek gpu_probe` 一次性 / daemon `-E gpuprobe`）、spec 可调项、探针 ELF 重编流程（指向 probe/README.md）。

- [ ] **Step 3: 提交**

```bash
git add -f docs/
git commit -m "docs(gpuprobe): 只读契约登记 + 组件说明"
```

---

## Task 10: 现场回归（手动，参照 sichek-field-regression skill）

- [ ] **Step 1: 构建 linux/amd64 并部署到测试节点** `/tmp/sichek-test`（见记忆 `field_regression_testing`）。
- [ ] **Step 2: 正例**——在记忆 `pattern_nccltest_fail_gpu_reset_required` 那类 reset-required 卡上跑 `sichek gpu_probe`，期望该卡 FAIL→Critical。
- [ ] **Step 3: 负例**——健康节点跑，期望空闲卡 PASS、无误报。
- [ ] **Step 4: 忙节点**——有业务在跑的卡，期望 skip、**不干扰业务**（对比业务吞吐前后无明显波动）。
- [ ] **Step 5: daemon 路径**——`-E gpuprobe` 起 daemon，抓 `sichek_gpuprobe_*` 指标 + 确认 K8s annotation 出 `gpuprobe` 段（用 `-A scitix.ai/sichek-test` 测试 key，见记忆 `bug_stale_annotation_after_reboot`）。
- [ ] **Step 6: 产出 Markdown 回归报告**，键到验收标准。

---

## Self-Review 记录

- **Spec 覆盖**：探针改造(§6)→T1；打包入仓(§6)→T1b/T8；config+spec(§13)→T2；门控(§7)→T3；超时/回收(§9)→T3;边界(§10)→T3/T6;定级+去抖(§8)→T4;metrics(§11.4)→T5;组件(§4/§5)→T6;接线(§11)→T7;CLI(§12)→T7;文档(§11.5)→T9;测试(§14)→T2/T3/T4;回归→T10。✅ 全覆盖。
- **类型一致性**：`OutcomeXxx` 常量在 collector 定义，checker/metrics 引用同名；`GpuProbeInfo`/`GpuProbeResult` 字段贯穿一致;`GpuProbeSpec` 字段名 config↔collector↔checker 对齐。✅
- **待实现时核对的外部依赖**（已在对应步骤标注）：`metrics.GaugeVecMetricExporter` 方法名、`common.LoadUserConfig` 签名、goreleaser 真实结构、默认 user config 文件路径。这些是"按仓库现状核对后微调"，非占位符。
- **默认关机制**：靠"不进 DefaultComponents"实现，经 `-E gpuprobe` 开启——与 sichek 现有 enable/ignore 机制一致，未发明新开关。✅
