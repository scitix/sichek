# check_ib_mezz_name — mezz 卡命名一致性检查

日期:2026-09-02
组件:`components/infiniband`
分支:`feat/ib-mezz-name-checker`

## 目标

新增一个 InfiniBand 检查项,校验每张 **mezz 卡**(内部 IB mezzanine 卡)的 RDMA 设备名是否符合 `mezz_<k>` 命名约定。命名不符说明上游 `rdma-env-pre` 的 `interface-naming` 没有生效,判 **Critical**。

## mezz 卡的判据(硬编码)

依据 `rdma-env-pre` 文档 `docs/mezz-card-identification.md`:**mezz 的判据是 board_id(固件 PSID)**。PCI vendor:device(`0x1021`)与 CX7 相同,不能用来区分,只有 board_id 能分开。

board_id **跨 GPU 代不同**,因此以硬编码集合锚定:

```go
var MezzBoardIDs = map[string]bool{
    "NVD0000000079": true, // B300 (gpu-10-220-55-90 实测)
    "MT_0000001121": true, // B200 (gpu-10-220-55-7 实测)
}
```

新代出货需在此扩充。注意上游 `rdma-env-pre` 的 `internal-mezz-ib.json` 目前只声明 `NVD0000000079`,B200 的 `MT_0000001121` 由真机 B200 节点实测得到(4 张命名为 `mezz_0..3` 的内部卡,board_id 一致,与 compute HCA `MT_0000000838` 分离),应与上游 owner 对齐后同步维护。

## 关键约束:不依赖 spec、不走 spec-gated 路径

IB 组件在 `newInfinibandComponent` 里先 `LoadSpec`(→ `FilterSpec`);当某个检测到的 board_id 在 HCA spec 里查不到时,`FilterSpec` 返回 `spec not found for board IDs`,组件进入 `initError`。此时 `HealthCheck` 开头 `if c.initError != nil { return c.reportInitErrorResult() }` 直接返回,**collect 与所有 checker 都不跑**。mezz 的 board_id `NVD0000000079` 恰恰常常不在下发 spec 里(它本就不该有 HCA spec),于是普通 checker 在 mezz 节点上根本跑不到。

**本设计不修改 `FilterSpec`**。mezz 命名检查做成一条**完全不依赖 spec、也不依赖重采集器**的独立路径:

- IB 设备名 = `/sys/class/infiniband/<name>/` 的目录名;
- board_id = `/sys/class/infiniband/<name>/board_id`;
- 端口状态(仅展示用)= `/sys/class/infiniband/<name>/ports/1/phys_state`。

纯 sysfs 扫描即可完成判定,无需 `InfinibandSpec`、无需 `NewIBCollector`。

## 判定逻辑

函数 `mezzNamingResultAt(sysRoot string) *common.CheckerResult`(默认 `sysRoot=/sys/class/infiniband`):

1. 遍历 `sysRoot` 下每个 IB 设备目录 `<name>`;
2. 读 `<name>/board_id`,`TrimSpace` 后 `== MezzBoardID` 才纳入(非 mezz 全忽略);
3. 名字匹配正则 `^mezz_\d+$` → 命名对上;否则记为不符;
4. **有任一 mezz 命名不符 → Status=Abnormal / Level=Critical**;无 mezz 卡、或全部对上 → Normal;
5. Detail 逐张列出映射,格式对齐用户样例:`mezz_0 port 1 ==> mezz0 (Down)`
   - netdev 名 `mezz0` 由 `mezz_0` 约定推导(`mezz_`→`mezz`),**仅展示,不参与校验**;
   - `(Down)` 由 `phys_state != LINK_UP`(sysfs 返回如 `5: LinkUp` / `2: Polling` / `3: Disabled`)推导;
   - 命名不符的行额外标 `expected mezz_<k>`;
6. `Device` = 命名不符的设备名(逗号连接),供上层聚合定位。

结果模板放进 `InfinibandCheckItems`:
- `Level = consts.LevelCritical`
- `ErrorName = "IBMezzNameMismatch"`
- `Suggestion` = 提示 `rdma-env-pre interface-naming` 未生效,mezz 的 RDMA 设备应命名为 `mezz_<k>`

## 两条运行路径(方案 A)

1. **正常路径**:注册为普通 IB checker(`NewIBMezzNameChecker`),`Check(ctx, data)` 忽略传入 data、直接调 `mezzNamingResultAt` 默认根,随 `common.Check` 与其它 checker 一起跑。
2. **initError 路径**:在 `HealthCheck` 的 `if c.initError != nil` 分支里调 `MezzNamingResult()`,**仅当结果为 abnormal 时**才 append 进 `reportInitErrorResult()` 的 `Checkers`。原因:initError 分支的 `PrintInfo` 会把结果里每个 checker 无条件标红并打 `ErrorName`,若把一个 Normal/无-mezz 的结果也 append 进去,会被渲染成误导性的红色"失败"行。因此只在 mezz 命名**真的失败**时才在此路径浮现。

两条路径调用同一个 `mezzNamingResultAt`,判定完全一致。**不改 `FilterSpec`;只在 HealthCheck 的 initError 分支加几行独立调用。**

## 改动文件

| 文件 | 改动 |
|---|---|
| `components/infiniband/config/check_items.go` | 加 `CheckIBMezzName`、`MezzBoardID` 常量 + `InfinibandCheckItems` 模板项 |
| `components/infiniband/checker/mezz_name.go` | 新增:`mezzNamingResultAt` + `MezzNamingResult`(默认根)+ `IBMezzNameChecker`/`NewIBMezzNameChecker`/`Name`/`Check` + `^mezz_\d+$` 正则 |
| `components/infiniband/checker/checker.go` | `checkerConstructors` 加 `config.CheckIBMezzName: NewIBMezzNameChecker` |
| `components/infiniband/infiniband.go` | `HealthCheck` 的 initError 分支追加 mezz 结果并合并 Status/Level |
| `components/infiniband/checker/mezz_name_test.go` | 表驱动测试(见下) |

## 测试

`mezzNamingResultAt(t.TempDir())` 造假 sysfs:

- mezz 命名正确(`mezz_0`/`mezz_1`,board_id=NVD79)→ Normal;
- mezz 命名错误(`mlx5_9`,board_id=NVD79)→ Abnormal / Critical,Device 含 `mlx5_9`;
- 无 mezz(只有普通 CX7 board_id)→ Normal;
- 混合(一张对、一张错)→ Abnormal / Critical,只报错的那张;
- phys_state 映射:`5: LinkUp`→无 `(Down)` 后缀,`2: Polling`→`(Down)`。

## 非目标 / 已知边界

- 不校验 netdev 名(`mezz<k>`),只校验 RDMA 名 `mezz_<k>`(按用户确认)。
- 不校验 mezz 卡数量(避免"少一张"类误报);只对**已出现**的 mezz 设备判命名。
- 不修改 `FilterSpec` 的缺-spec 拦截逻辑;mezz 缺 HCA spec 仍会让其它 IB checker 处于 initError,但本检查照常运行。
- Critical 级别按用户确认;语义上 mezz 命名不符更多是 `rdma-env-pre` 未跑到位的信号,若后续认为 cordon 过重可降 Warning(仅改模板 `Level`)。
