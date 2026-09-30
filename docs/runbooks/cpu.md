# CPU 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

---

## cpu-mce-uncorrected — CPUMCEUncorrected (Critical)

**1. 是什么** — 校验是否检测到未纠正的 Machine Check Exception(Uncorrected MCE)。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=cpu-mce-uncorrected component=cpu`,ErrorName 为 `CPUMCEUncorrected`(下为示意形状,非精确字符串)。

```
checker=cpu-mce-uncorrected component=cpu  ErrorName=CPUMCEUncorrected  Level=Critical
```

**3. 常见根因**

- CPU/内存子系统发生了硬件层面的严重错误(如内存 ECC 不可纠正、缓存/互联总线错误)。
- 主板供电异常或 CPU 本体存在硬件缺陷。
- 极端环境(过温、震动导致接触不良)诱发的一次性硬件故障。

**4. 确认命令(只读)**

```bash
journalctl -k | grep -i mce                        # 内核日志里的 MCE 记录
dmesg | grep -i "Machine check"                    # dmesg 中的 MCE 事件
ras-mc-ctl --summary                                # RAS 汇总(若已安装 rasdaemon)
mcelog --client                                     # 若使用 mcelog 守护进程
```

**5. 处置**

- 节点侧可自愈/重配:无(Uncorrected MCE 属硬件层错误,节点侧无法软件修复,只能记录/上报)。
- 该换硬件/排期维护(升级硬件团队):按 Suggestion「Uncorrected MCE detected; this indicates a serious hardware error. Schedule maintenance」安排维护窗口,排查/更换故障 CPU 或内存模组。

**6. 级别与升级** — Critical,cordon 节点尽快修;属硬件错误,升级硬件团队并排期维护。

---

## cpu-performance — CPUPerfModeNotEnabled (Warning)

**1. 是什么** — 校验所有 CPU 核心是否运行在 `performance` 调频模式。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=cpu-performance component=cpu`,ErrorName 为 `CPUPerfModeNotEnabled`(下为示意形状,非精确字符串)。

```
checker=cpu-performance component=cpu  ErrorName=CPUPerfModeNotEnabled  Level=Warning
```

**3. 常见根因**

- 系统默认调频策略非 performance(如 powersave/ondemand),未在上线时统一配置。
- 重启后 cpufreq governor 设置未持久化,回落到默认值。
- 部分 CPU 核心的 governor 被单独修改,导致核心间不一致。

**4. 确认命令(只读)**

```bash
cat /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor   # 逐核查看当前 governor
cpupower frequency-info                                      # 若已安装 cpupower
```

**5. 处置**

- 节点侧可自愈/重配:执行 `echo performance > /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor` 将所有 CPU 设为 performance 模式;理想情况下应由上线流程自动完成(sichek 自身在该 checker 中也会尝试自动写入并复检,失败才报警)。

**6. 级别与升级** — Warning,cordon 节点择期修;属节点侧配置项,无需升级硬件团队。

---

## clock-sync-service — ClockSyncServiceNotRunning (Warning)

**1. 是什么** — 校验节点上是否有 PTP 或 NTP 时钟同步服务在运行。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=clock-sync-service component=cpu`,ErrorName 为 `ClockSyncServiceNotRunning`(下为示意形状,非精确字符串)。

```
checker=clock-sync-service component=cpu  ErrorName=ClockSyncServiceNotRunning  Level=Warning
```

**3. 常见根因**

- ptp4l/chronyd/ntpd 均未安装或服务未启动。
- 服务崩溃退出后未被拉起(无自动重启机制)。
- 节点重装/改镜像后遗漏时钟同步组件的初始化配置。

**4. 确认命令(只读)**

```bash
systemctl status chronyd ntpd ptp4l                # 看三种时钟同步服务状态
chronyc tracking                                   # 若使用 chronyd,查同步详情
timedatectl status                                 # 系统总体时间同步状态
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion「Ensure ptp4l, chronyd, or ntpd service is running for clock synchronization」确认并启动对应服务,核对开机自启配置。

**6. 级别与升级** — Warning,cordon 节点择期修;属节点侧服务配置,无需升级硬件团队。

---

## clock-sync-offset — ClockSyncOffsetHigh (Warning)

**1. 是什么** — 校验时钟同步偏移量是否在可接受范围内。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=clock-sync-offset component=cpu`,ErrorName 为 `ClockSyncOffsetHigh`(下为示意形状,非精确字符串)。

```
checker=clock-sync-offset component=cpu  ErrorName=ClockSyncOffsetHigh  Level=Warning
```

**3. 常见根因**

- 上游时钟源(PTP master/NTP server)本身偏移较大或不可达。
- 网络时延抖动大,PTP/NTP 收敛慢或反复失步。
- 本地 PTP/NTP 配置不当(如源列表错误、硬件时间戳未启用)。

**4. 确认命令(只读)**

```bash
chronyc tracking                                   # System time 偏移量、频差
chronyc sources -v                                 # 各时钟源状态与偏移
pmc -u -b 0 'GET CURRENT_DATA_SET'                 # 若使用 ptp4l,查 offset(需 linuxptp 工具)
```

**5. 处置**
- 节点侧可自愈/重配:按 Suggestion「Check PTP/NTP configuration; high clock offset may cause distributed training issues」核对 PTP/NTP 配置(时钟源、网络路径),必要时重启同步服务待其重新收敛。

**6. 级别与升级** — Warning,cordon 节点择期修;属节点侧配置/网络问题,一般无需升级硬件团队,时钟源侧问题可联系相关团队。

---

## cpu-mce-corrected — CPUMCECorrectedHigh (Warning)

**1. 是什么** — 校验已纠正的 Machine Check Exception(Corrected MCE)数量是否处于高位。

**2. 典型输出**

失败态在 `sichek i` 中呈现为 `checker=cpu-mce-corrected component=cpu`,ErrorName 为 `CPUMCECorrectedHigh`(下为示意形状,非精确字符串)。

```
checker=cpu-mce-corrected component=cpu  ErrorName=CPUMCECorrectedHigh  Level=Warning
```

**3. 常见根因**

- 内存/缓存存在间歇性可纠正错误,尚未升级为不可纠正错误,但数量持续走高。
- CPU 或内存模组存在早期硬件劣化迹象。
- 环境因素(温度、供电波动)导致纠正错误率偏高。

**4. 确认命令(只读)**

```bash
journalctl -k | grep -i mce                        # 内核日志里的 corrected MCE 记录
ras-mc-ctl --summary                                # RAS 汇总(若已安装 rasdaemon)
edac-util -v                                        # EDAC 纠错计数(若已安装)
```

**5. 处置**

- 节点侧可自愈/重配:按 Suggestion「High corrected MCE count detected; monitor for increasing errors and schedule preventive maintenance」持续观察计数趋势,暂不影响业务可先监控。
- 该换硬件/排期维护(升级硬件团队):计数持续上升或短时间内快速增长 → 排期维护,更换存在劣化迹象的 CPU/内存模组,避免升级为 Uncorrected MCE。

**6. 级别与升级** — Warning,cordon 节点择期修;计数持续恶化时升级硬件团队并排期维护。
