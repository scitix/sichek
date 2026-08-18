# gpuprobe — GPU 功能自检主动探测组件设计

- 日期：2026-08-13
- 状态：设计已批准，待写实现计划
- 分支（建议）：`feat/gpuprobe-active-probe`

## 1. 背景与动机

sichek 现有 21 个组件几乎都是**被动采集指标再判定**（NVML / sysfs / ibstat / dmesg → checker）。这类检查能发现"指标异常"，但发现不了"卡看着一切正常、可真让它算就算错/算不出来"的故障——例如记忆里 `pattern_nccltest_fail_gpu_reset_required`：nvidia-smi 全空闲、指标无异常，实际某卡已 reset-required，只有真跑一次 all_reduce 才暴露。

仓库里已有主动探测的雏形——`components/infiniband/perftest/`（`local_ibtest.go` / `local_rocetest.go`）+ CLI 子命令 `nccl_perftest` / `ib_perftest` / `roce_perftest`：它们 `exec` 外部二进制、解析 stdout、跟阈值比、返回 `common.Result`。但 perftest **不是** `common.Component`——没有 daemon、annotation、metrics，只能 CLI 一次性跑。

本设计新增 `gpuprobe` 组件，把"主动在 GPU 上跑一个小计算核、验证卡真的能算且算得对"做成**正式的 `common.Component`**，接入 daemon / K8s annotation / Prometheus，成为 sichek 首个受管的主动探测组件。

种子探针来自用户提供的 CUDA 单卡功能自检脚本（malloc → memcpy → kernel `c=a*2+b` → 逐元素校验），本设计对其做退出码语义化改造。

## 2. 目标 / 非目标

**目标**
- 周期性（默认 10 分钟）在**空闲** GPU 上跑一个极小的 CUDA 计算核，验证计算通路健康。
- 绝不干扰在跑的业务：忙卡一律跳过（空闲门控 + 探针内部自查双防线）。
- 探测失败按 sichek 级别语义映射（FAIL/timeout=Critical，环境/执行错=Warning，忙=跳过）。
- 结果进 daemon、K8s annotation、Prometheus，与其他组件一致。
- 纯 Go 主构建不变（离线可建、无 nvcc 依赖）；探针 ELF 预编译入仓、随包发布。

**非目标**
- 不做多卡/跨节点集合通信压测（那是 perftest 的 nccl/ib 职责）。
- 不做性能基线（带宽/算力）判定，只做功能"对/错/超时"判定。
- 不支持 MIG 实例探测（首版跳过并标注，列为后续）。
- 不在构建链引入 CUDA toolchain（探针预编译入仓）。

## 3. 关键决策（brainstorming 结论）

| # | 决策点 | 结论 |
|---|---|---|
| 1 | 探测对象 | GPU 功能自检（跑 kernel 验证算得对） |
| 2 | 运行模式 | daemon + 空闲门控（忙卡跳过） |
| 3 | 探测机制 | 内置自写 CUDA 探针 ELF，exec 后解析退出码 |
| 4 | 探针打包 | 预编译 ELF 入仓（amd64/arm64）+ goreleaser 随包发布到 `/var/sichek/bin/gpu_probe` |
| 5 | 定级 | FAIL/timeout=Critical，env/exec 错=Warning，忙=跳过(Info) |
| 6 | 默认开关 | **默认关**，需 user config 显式 enable（CLI 子命令始终可手动跑） |
| 7 | 调度间隔 | 默认 `query_interval: 600s`（10 分钟） |
| 8 | 去抖锁定 | 可配 `fail_consecutive_threshold`，**默认 1**（单轮 FAIL 即 Critical） |

架构方案选 **A：全新标准组件**（照 CLAUDE.md "新组件照抄现有组件"），而非薄壳包 perftest（B）或塞进 nvidia 组件（C，会污染其只读语义）。

## 4. 组件结构

```
components/gpuprobe/
├── gpuprobe.go                     // 实现 common.Component，嵌 CommonService，进 daemon
├── collector/
│   └── collector.go                // 枚举 GPU → 空闲门控 → exec 探针 → 收退出码
├── checker/
│   ├── check_gpu_probe.go          // outcome → CheckerResult（定级）
│   └── check_gpu_probe_test.go
├── config/
│   ├── config.go                   // GpuProbeUserConfig（interval/cache/metrics 开关）
│   ├── spec.go                     // spec 加载
│   ├── check_items.go              // CheckerResult 模板
│   └── default_gpuprobe_spec.yaml  // 门控阈值/超时/level/探针路径/去抖阈值
├── metrics/
│   └── metrics.go                  // sichek_gpuprobe_* gauges
├── probe/
│   ├── gpu_probe.cu                // 探针源码（退出码语义化改造版）
│   ├── Makefile                    // nvcc 编译命令（不进主构建）
│   └── README.md                   // 重编/SM 架构说明
└── bin/
    ├── gpu_probe.amd64             // 预编译 ELF（入仓）
    └── gpu_probe.arm64
```

## 5. 数据流

一次 `HealthCheck` tick：

```
collector.Collect(ctx):
  0. 重入门闩：atomic CAS "probing"，上一轮未完则本轮直接返回上次结果（不叠加）
  1. nvidia-smi -L 枚举 GPU；无 GPU / 无 nvidia-smi → 返回空 Info，组件 Normal
  2. 逐卡读门控信号：
       nvidia-smi --query-compute-apps=pid -i <idx>           // 有进程?
       nvidia-smi --query-gpu=utilization.gpu,memory.used,memory.total -i <idx>
     idle = 无计算进程 AND free% >= min_free_mem_pct AND util <= max_gpu_util_pct
  3. busy 卡：outcome=skip（不启探针）
     idle 卡：exec.CommandContext(ctx, probe, "-d", idx) 带 per-probe 超时，串行
              收 exitCode + stdout
  4. 汇成 GpuProbeInfo{ PerGPU: []GpuProbeResult }

checker.Check(GpuProbeInfo):
  逐卡 outcome → CheckerResult（含去抖阈值判定）

gpuprobe.HealthCheck: Collect → common.Check → 写 cache → 返回 Result → daemon 出 annotation/metrics
```

### 数据结构

```go
// collector
type GpuProbeResult struct {
    Index      int    // GPU 序号
    BDF        string // PCI BDF，便于定位（如 bb:00）
    State      string // "idle" | "busy"
    Outcome    string // "pass" | "fail" | "skip" | "env_err" | "exec_err" | "timeout"
    ExitCode   int
    DurationMs int64
    Detail     string // stdout 摘要 / 跳过原因 / 错误信息
}

type GpuProbeInfo struct {
    Time   time.Time
    PerGPU []GpuProbeResult
}
// GpuProbeInfo 实现 common.Info（JSON()）
```

## 6. 探针改造（gpu_probe.cu）

基于用户提供的 .cu，核心改动是**退出码语义化**（原脚本"跳过"和"通过"都返回 EXIT_SUCCESS，退出码分不清）：

| 退出码 | 含义 | 触发 |
|---|---|---|
| `0` PASS | 算得对 | kernel 结果逐元素校验通过 |
| `1` FAIL | 算错了 | 结果 mismatch |
| `2` SKIP | 主动让路 | free% < min-free-pct / `cudaErrorMemoryAllocation` / `cudaErrorDevicesUnavailable` |
| `3` ENV_ERR | CUDA 环境错 | 其余 CUDA API 失败（原 `CHECK_CUDA` 宏 `exit(FAILURE)` 路径改成 `exit(3)`） |

其他改动：
- `CHECK_CUDA` 宏中 `exit(EXIT_FAILURE)` → `exit(3)`，把"环境错(3)"与"算错(1)"分离。
- 关键信息打成 **stdout 单行结构化**：`RESULT=PASS device=0 free_mb=... dur_ms=...`；sichek 主认退出码，stdout 仅作 Detail 佐证。
- 保留 `-d <idx>`；新增可选 `--min-free-pct`（默认 50，由 sichek 从 spec 传入保持一致），作为探针内部自查阈值（第二道防线）。
- 保持极小极快（1M int × 3 ≈ 12MB，单 kernel），压低占卡窗口与争抢。

### 编译与链接
- nvcc 预编译两份 ELF：`gpu_probe.amd64` / `gpu_probe.arm64`，提交到 `components/gpuprobe/bin/`。
- **静态链 `-lcudart_static`**：runtime API 不依赖外部 libcudart；运行期只依赖驱动的 `libcuda.so.1`（GPU 节点必有，DaemonSet 由 nvidia runtime 注入）。探针因此**不绑 CUDA 版本**、跨节点通用。
- `-arch` 覆盖目标 SM（在 probe/README.md 记录，含 Hopper/Blackwell）。
- `probe/Makefile` 单独编译，**不接入主 `make`**——主流程保持纯 Go、离线可建。

## 7. 空闲门控（busy 定义）

任一信号命中即 busy（跳过）；idle 须三条全满足。**宁可漏测，绝不抢占业务卡。**

| 信号 | 数据源 | 默认阈值 | 作用 |
|---|---|---|---|
| ① 有计算进程 | `nvidia-smi --query-compute-apps=pid,used_memory -i <idx>` | 有任意进程 → busy | 最硬的地面真相；挡住 EXCLUSIVE_PROCESS 独占 |
| ② 显存占用 | `--query-gpu=memory.used,memory.total` | free% < 50% → busy | 兜住进程查询被 namespace 挡住但显存已占 |
| ③ 利用率 | `--query-gpu=utilization.gpu` | util > 10% → busy | 廉价加固；瞬时噪声，不单独作数 |

**双防线兜竞态**：读门控到 exec 探针之间可能有业务起跑——
1. sichek 侧门控先拦；
2. 探针 ELF 建 context 时再自查一次（free% / `cudaErrorDevicesUnavailable` → exit 2）。
撞窗口时探针自报 skip 而非 FAIL。

## 8. 判定定级

checker `check_gpu_probe.go`，逐卡一条 CheckerResult：

| outcome | 退出码 | Status | Level | ErrorName | 语义 |
|---|---|---|---|---|---|
| pass | 0 | Normal | — | — | 卡算得对 |
| fail | 1 | Abnormal | Critical¹ | `GPUProbeFailed` | 空闲卡结果算错 → cordon |
| skip | 2 | Normal | Info | — | 卡忙，主动让路，不告警 |
| env_err | 3 | Abnormal | Warning | `GPUProbeEnvError` | CUDA 环境异常，排期查 |
| timeout | (被杀) | Abnormal | Critical¹ | `GPUProbeTimeout` | 空闲卡连小 kernel 都跑不完，疑似 hang |
| exec_err | (启动失败/缺失) | Abnormal | Warning | `GPUProbeExecError` | 探针不在/不可执行/无 GPU 工具 |

¹ 受 `fail_consecutive_threshold` 去抖控制：默认 1（单轮即 Critical）；设为 N 时，同一卡需连续 N 轮 FAIL/timeout 才升 Critical，之前轮次先报 Warning 观察。连续计数按 GPU BDF 维护，命中 pass/skip 清零。

`Device` 字段填 `GPU<idx> <BDF>`（如 `GPU6 bb:00`），便于按记忆里 `lh-g23-191 GPU6` 的方式定位。

## 9. 超时与资源回收

daemon 长时运行（每 10 分钟一轮 × 每卡一子进程），任何泄漏都会累积，必须严格回收。

| 对象 | 处理 |
|---|---|
| 子进程 reap | 每次 exec 必 `cmd.Wait()`（含超时路径），杜绝僵尸 |
| 进程组 | `SysProcAttr{Setpgid:true}`；超时 kill 整个进程组（`syscall.Kill(-pgid, SIGKILL)`），清孙进程 |
| 不可杀 hang（D 态） | SIGKILL 杀不掉卡在内核态的 CUDA 进程。kill 后另起 goroutine 做**带宽限二段 Wait**（grace 默认 5s）；到点未 reap → 记 timeout、记日志、**继续下一轮**，绝不让 daemon 卡死等它 |
| GPU 显存/context | 进程一死驱动自动回收（含被 SIGKILL 者），sichek 不做 device 侧清理；唯一残留是 D 态 hang，而那正是要报的故障 |
| timer/goroutine | 每轮 `context.WithTimeout` 的 `cancel` 必 `defer` |
| FD/pipe | 固定 buffer + `Output()`/`Wait()`，pipe 随 Wait 关闭 |
| 重入 | atomic "probing" 门闩：上一轮未完（hang grace 期）则本轮 tick 跳过，绝不对同卡并发探针 |
| daemon Stop | 探针 context 挂在组件 ctx 上，`Stop()`/重启时取消，杀在飞子进程，不留孤儿 |

超时用 `exec.CommandContext` + per-probe `context.WithTimeout(probe_timeout_sec，默认 30s)`；逐卡串行，单卡超时不影响其余卡。整轮上限 = 卡数 × timeout（8×30=240s）< query interval（600s）。

## 10. 边界情况

| 场景 | 行为 |
|---|---|
| 无 GPU / 无 nvidia-smi | collector 返回空 Info，组件 Normal，不报错 |
| 驱动未起 / nvidia-smi 报错 | exec_err Warning，不崩 |
| 全部卡忙 | 全 skip，组件 Normal，Detail 记"本轮 0 卡受测" |
| MIG 卡 | 跳过 + Info 标注"MIG 暂不支持"（后续支持） |
| EXCLUSIVE_PROCESS 独占但进程查询漏 | 探针拿不到 context → exit 3 → env_err Warning（不假 FAIL） |
| 探针缺失/不可执行/架构不符/libcuda 缺 | exec 失败 → exec_err Warning |
| 部分卡忙部分空闲 | 逐卡各自结果，混合 |
| 重启后旧告警残留 | 复用已有"启动清空 annotation"修复，gpuprobe 同样被清+重写 |

## 11. 接线（防"新组件被静默丢弃"）

参照记忆 `annotation_schema_must_track_components`——新组件不接全套线，其 issue 会被 annotation / snapshot 静默丢弃。必接：

1. `consts/consts.go`：`ComponentNameGPUProbe = "gpuprobe"`、checker ID 常量、默认 spec 文件名常量。
2. `service/info.go`：`nodeAnnotation` 结构体加字段 + **两个 switch 都加 case**。
3. daemon 组件注册表加 gpuprobe；**不进 `DefaultComponents`**（默认关，决策 6）。
4. `metrics/`：
   - `sichek_gpuprobe_probe_status{gpu,bdf}`（0=pass,1=fail,2=skip,3=env_err,4=exec_err,5=timeout）
   - `sichek_gpuprobe_duration_ms{gpu,bdf}`
   - `sichek_gpuprobe_last_run_ts{gpu,bdf}`
5. `docs/write-operations-audit.html` + 组件文档显式登记："gpuprobe 会在空闲卡上跑 kernel"——sichek 首个正大光明的主动占卡能力，默认关。

## 12. CLI

`cmd/command/component/gpuprobe.go` → 子命令 `gpu_probe`（一次性）：
- 默认带空闲门控；
- `--force`：跳过门控，强制逐卡深测（人工排障）；
- `-d <idx>`：只测指定卡。

与其他组件 CLI 面一致，不违背"daemon + 门控"的默认运行模式。

## 13. 配置

**user config**（`GpuProbeUserConfig`，运行时可调）：
```yaml
gpuprobe:
  enable: false            # 决策 6：默认关
  query_interval: 600s     # 决策 7：10 分钟
  cache_size: 5
  enable_metrics: true
```

**spec**（`default_gpuprobe_spec.yaml`，判定基线）：
```yaml
gpuprobe:
  probe_binary_path: /var/sichek/bin/gpu_probe
  probe_timeout_sec: 30
  kill_grace_sec: 5
  min_free_mem_pct: 50
  max_gpu_util_pct: 10
  skip_if_compute_apps: true
  skip_mig: true
  fail_consecutive_threshold: 1   # 决策 8：默认单轮即 Critical
  fail_level: Critical
  env_error_level: Warning
```

## 14. 测试

testify + 表驱动 + `t.TempDir`，**不碰真 GPU**：
- **checker**：喂造好的 `GpuProbeInfo`（各 outcome + 去抖阈值场景）验级别映射与连续计数清零。
- **collector**：shell stub 假探针（按参数返回各退出码 / 挂起触发超时）+ 假 `nvidia-smi`（`PATH` 注入），验门控三信号判定、outcome 归类、超时 kill+reap、重入门闩。
- **无 GPU**：假 `nvidia-smi -L` 返回空 → 组件 Normal。

真机回归（记忆 `field_regression_testing`）：正例节点用记忆里 `pattern_nccltest_fail_gpu_reset_required` 的 reset-required 卡验 FAIL→Critical；健康节点验 pass；忙节点验 skip 不干扰业务。

## 15. 实现顺序（供写计划参考）

1. 探针 .cu 退出码改造 + Makefile/README + 预编译入仓两份 ELF。
2. config（user + spec + check_items）。
3. collector（枚举 + 门控 + exec + 超时/回收 + 重入）。
4. checker（定级 + 去抖）。
5. 顶层 gpuprobe.go（Component + CommonService）。
6. 接线（consts / service/info.go / daemon 注册 / metrics）。
7. CLI 子命令。
8. goreleaser + 安装路径打包。
9. 测试（checker/collector/无 GPU）。
10. 文档（write-operations-audit 登记 + 组件文档）。
11. 真机回归。
