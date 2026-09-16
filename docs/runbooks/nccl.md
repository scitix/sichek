# NCCL / 通信测试排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

---

## NCCLTimeout — NCCLTimeout (Fatal)

**1. 是什么** — 用户任务运行中发生 NCCL 通信超时(集合通信 all-reduce 等长时间无响应)。

**2. 典型输出**

```
NCCL timeout occurred during user task
（示意:任务日志出现 Watchdog ... NCCL timeout / Some NCCL operations have failed or timed out;设备 :pytorch-master-0)
```

**3. 常见根因**

- 某 rank 掉队/卡死(常由单卡 Xid、GPU hang 引起)。
- 网络轨道故障(IB/RoCE 掉链路),集合通信一直等不到对端。
- bootstrap 网卡选错,通信建链失败。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -i xid                             # 找肇事卡的 Xid
sichek gpu                                          # 排查 GPU 侧异常
sichek infiniband                                  # 排查 IB/RoCE 轨道
# 检查各 rank 进程是否存活、卡在同一集合通信
```

**5. 处置**

- Fatal → 终止任务重投;定位掉队 rank(交叉参考 nvidia.md Xid / hang.md / 下方 nccltest)。
- 若根因是硬件(掉卡/掉链路),按对应 checker 处理并换硬件。

**6. 级别与升级** — Fatal:杀任务重投,**不 cordon**;但若根因是硬件(掉卡/掉链路),对应 checker(如 IBLost / GPU 类)会 cordon 该节点并走硬件团队升级。

---

## nccltest — 通信压测失败 (诊断工具)

**1. 是什么** — sichek 主动跑 NCCL all_reduce_perf 压测(`sichek nccl`),验证多卡集合通信是否健康;这是主动诊断工具,非常驻 checker。

**2. 典型输出**

```
（示意,FAIL 时任一)
CUDA device busy or unavailable
IBV_WC_RETRY_EXC_ERR
NCCL bootstrap bind failed: Cannot assign requested address (EADDRNOTAVAIL)
```

**3. 常见根因**

- 某卡 reset-required(真阳性):nvidia-smi 看着空闲但报 "CUDA device busy or unavailable" → dmesg 查 Xid 94→95→154(GPU Reset Required);逐卡 all_reduce_perf 定位(参考 nvidia.md gpu-recovery-action / xid-95)。
- 双 IB fabric 节点 false-FAIL:NCCL 默认抓到存储 fabric 的 HCA → 跨子网 IBV_WC_RETRY_EXC_ERR;修=pin 算力 HCA(NCCL 排除 GPFS verbsPorts)。
- OVS/Spectrum-X 上 NCCL bootstrap 挑中 VRF 算力轨口 bind 失败(EADDRNOTAVAIL)→ 修=NCCL_SOCKET_IFNAME=bond0(管理网,非 VRF)。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -i <N>                               # 看 GPU Recovery Action
dmesg -T | grep -i xid                             # 找 Xid 94/95/154
# 逐卡跑 all_reduce_perf 定位肇事卡
ibstat                                             # 核对算力/存储 HCA
```

**5. 处置**

- 定位到肇事卡 → 按 nvidia.md 处理(reset / cold reset / 换卡)。
- 网卡选择类 → 设 `NCCL_SOCKET_IFNAME` / pin 算力 HCA(排除 GPFS verbsPorts)。
- 注意:dmesg 里 `name=sichek` 是连坐(被压测拖累)非肇事。

**6. 级别与升级** — 诊断工具本身不判级;定位到的根因按对应 checker 级别处理(如 GPU reset-required 走换卡升级,掉链路走 IB checker cordon)。
