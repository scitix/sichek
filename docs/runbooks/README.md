# sichek 排障知识库(SRE Runbook)

本目录面向 SRE:拿着 `sichek i` / `sichek <component>` 的报错,按 **checker 名** 对号入座,查「是什么 / 为什么 / 怎么确认 / 怎么修 / 何时升级」。

## 怎么用

1. 看 `sichek` 输出里的 `checker=<名字>` 字段,或 `Errors Events` 里的报错句子。
   例:`checker=check_ib_port_speed`。
2. 在下方索引表 Ctrl-F 搜该 checker 名 / ErrorName,点链接跳到对应模块文件的条目。
3. 每个条目分六段,照「处置」段操作;拿不准或需换硬件时按「级别与升级」段处理。

## 级别语义(权威:`../errors-categorization.md`)

| 级别 | 动作 | 是否 cordon 节点 |
|---|---|---|
| **Fatal** | 立即停止任务并重新提交(任务级) | **否** |
| **Critical** | Cordon 节点,尽快修复硬件/软件 | **是** |
| **Warning** | Cordon 节点,择期修复 | **是** |

> 注意:Critical 与 Warning **都会 cordon 节点**,区别只在修复紧迫度。Fatal 是杀任务重投,不 cordon。

## 组件 ↔ 文件对照

| 组件(`component=`) | 文件 | 状态 |
|---|---|---|
| infiniband | [infiniband.md](infiniband.md) | 已填满 |
| nvidia / gpu | [nvidia.md](nvidia.md) | 待填 |
| pcie / pcie_topo_check | [pcie.md](pcie.md) | 待填 |
| nccl / nccltest | [nccl.md](nccl.md) | 待填 |
| ethernet | [ethernet.md](ethernet.md) | 待填 |
| cpu | [cpu.md](cpu.md) | 待填 |
| gpfs | [gpfs.md](gpfs.md) | 待填 |
| hang | [hang.md](hang.md) | 待填 |
| transceiver | [transceiver.md](transceiver.md) | 待填 |

## InfiniBand checker 索引

| checker 名 | ErrorName | 级别 | 条目 |
|---|---|---|---|
| _(Task 3 填入 17 行)_ | | | |
