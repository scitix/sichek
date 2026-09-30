# GPU Hang / 计算卡顿排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

---

## GPUHang — GPUHang (Fatal)

**1. 是什么** — GPU 长时间无进展/挂起(sichek 通过 GPU 活动指标或内核日志判定为 hang)。

**2. 典型输出**

```
Errors Events: GPU Hang
（component=hang / gpuevents,ErrorName=GPUHang,Level=Fatal;示意:hang 常伴随 dmesg 里 Xid 报错)
```

**3. 常见根因**

- 某卡计算内核死锁/长时间无进展(常与 Xid 异常、driver reset 相关)。
- 显存/总线故障或掉卡后 kernel 卡死(参考 nvidia.md Xid 事件)。
- 任务侧代码死循环或同步等待某个已挂死的 rank(常与 nccltest FAIL / NCCLTimeout 关联)。

**4. 确认命令(只读)**

```bash
dmesg -T | grep -iE 'xid|hang'                     # 找 Xid 与 hang 线索
nvidia-smi -q -i <N>                               # 看单卡状态/Recovery Action
nvidia-smi dmon                                     # 实时利用率/时钟,看是否卡住
```

**5. 处置**

- 节点侧可自愈/重配:终止并重启任务;若同卡反复 hang → reset GPU。
- 该换硬件/升级:reset 后复现 → cold reset / reboot 复测;再复现 → 换卡,升级硬件团队。

真机经验:hang 常与 nccltest FAIL / Xid 关联,交叉参考 nvidia.md(Xid 事件)与 nccl.md 定位掉队/肇事卡。

**6. 级别与升级** — Fatal:杀任务重投,**不 cordon**;若定位到硬件(反复 hang 的卡),换卡走硬件团队升级。

---

## SmClkStuckLow — SmClkStuckLow (Fatal)

**1. 是什么** — SM 时钟长时间卡在低频,通常是卡顿/降频或卡死的征兆。

**2. 典型输出**

```
Errors Events: SM clock too low for long time
（component=hang / gpuevents,ErrorName=SmClkStuckLow,Level=Fatal;示意:SM Clock 长期远低于额定值)
```

**3. 常见根因**

- 触发了 throttle/clock event(温度/功耗墙/硬件降频,见 nvidia.md clock-events)。
- GPU 负载异常或散热/供电问题导致持续降频。
- 卡死前兆:计算停滞使 SM 时钟塌到最低档。

**4. 确认命令(只读)**

```bash
nvidia-smi -q -d CLOCK                              # 看 SM Clock 当前/额定值
nvidia-smi dmon                                     # 实时时钟与利用率
dmesg -T | grep -iE 'xid|hang'                     # 找并发的 Xid/hang
nvidia-smi -q -i <N>                               # 单卡温度/功耗/throttle 原因
```

**5. 处置**

- 节点侧可自愈/重配:检查是否有 throttle/clock event(见 nvidia.md clock-events)、GPU 负载与温度;终止任务;必要时 reset GPU。
- 该换硬件/升级:排除温度/功耗墙后仍持续卡低频、或伴随 hang/Xid → cold reset / reboot,再复现则换卡,升级硬件团队。

**6. 级别与升级** — Fatal:杀任务重投,**不 cordon**;若定位到硬件降频/故障,换卡走硬件团队升级。
