# Ethernet 排障

> 检索:在本页 Ctrl-F 搜 `sichek` 输出里的 `checker=` 名字或 ErrorName。
> 级别语义见 [README](README.md#级别语义权威errors-categorizationmd)。

Ethernet 组件按网络分层做体检,分 5 个 checker:`L1(Physical Link)`、`L2(Bond)`、`L3(LACP)`、`L4(ARP)`、`L5(Routing)`。每层的 `checker=` 名即上述括号全名,失败时 `ErrorName` 是该层下的具体子类(如 `LinkDown`、`BondDown`、`LACPRateMismatch`)。

> 级别说明:5 个 checker 在代码中均为 **Info** 级——即提示性,**不 cordon 节点**,也不属于 Fatal/Critical/Warning 那套 cordon 语义。它们标记的是管理网/Bond/LACP 配置与链路计数问题,面向"配置纠偏 + 观察趋势",通常自行修正或排期处理,不直接摘节点。多数 ErrorName 的 `Detail` 里已内嵌对应的确认命令与期望值。

---

## L1(Physical Link) — 物理链路层 (Info)

**1. 是什么** — 校验 Bond 底下每个物理网卡(slave)的链路层健康:是否 link up、速率是否匹配、以及 CRC/Carrier/Drop/Tx-timeout 等错误计数是否在增长。

**2. 典型输出**（`checker=L1(Physical Link)`,ErrorName 为下列子类之一,示意)

```
checker=L1(Physical Link) component=ethernet ErrorName=LinkDown
NIC eth2 link not UP. Command: ethtool eth2, Expected: Link detected: yes, Actual: not connected or unknown.
```

本层可能的 ErrorName:`NoBondInterface`(没有 bond 接口)、`LinkDown`(物理口没 up)、`SpeedMismatch`(速率与 spec 不符)、`TxTimeout`(内核日志有 tx timeout)、`CRCErrorsGrowing`、`CarrierErrorsGrowing`、`DropsGrowing`(RX CRC / Carrier / 丢包计数在两次体检间增长)。

**3. 常见根因**

- 线缆/光模块/口坏或没插好 → `LinkDown` / CRC、Carrier 增长。
- 网卡与交换机口速率协商不符 → `SpeedMismatch`。
- 驱动/固件问题或链路抖动 → `TxTimeout`、Drop 增长。
- Bond 未配置或未起来 → `NoBondInterface`。

**4. 确认命令(只读)**

```bash
ls /proc/net/bonding/                 # 有无 bond
ethtool <slave>                       # Link detected / Speed
ip -s link show <slave>               # RX errors / dropped / carrier 计数
ethtool -i <slave>                    # 驱动/固件版本
dmesg | grep -iE 'eth|mlx|link'       # tx timeout 等内核日志
```

**5. 处置**

- 节点侧可修:速率不符 → 核对交换机口与网卡配置;计数增长伴随抖动 → 检查线缆/光模块,重插或更换已知好线;`NoBondInterface` → 核对 `/etc/netplan` 或 `network-scripts` 的 bond 配置。
- 该换硬件(升级网络/硬件团队):换线后仍 `LinkDown` 或 CRC 持续增长 → 疑似口/卡/交换机侧硬件,升级处理。

**6. 级别与升级** — Info(提示性,不 cordon)。物理层错误若持续增长会实际影响业务带宽,建议结合 transceiver.md(光模块)与交换机侧一并排查后再决定是否人工摘节点。

---

## L2(Bond) — 链路聚合(Bonding)层 (Info)

**1. 是什么** — 校验 Bond 接口本身的状态与参数:MII 状态是否 up、slave 数量、MTU、xmit_hash_policy、miimon/updelay/downdelay 等 bonding 参数是否符合 spec,以及 active slave 是否频繁切换、link failure 是否增长。

**2. 典型输出**（`checker=L2(Bond)`,ErrorName 示意)

```
checker=L2(Bond) component=ethernet ErrorName=BondDown
Overall status of bond interface bond0 mismatch. Command: cat /proc/net/bonding/bond0, Expected: MII Status: up, Actual: mismatch (possibly down).
```

本层可能的 ErrorName:`NoBondInterface`、`BondingMissing`(spec 里的 bond 在 `/proc/net/bonding` 不存在)、`BondDown`(整体 MII 非 up)、`MTUMismatch`、`XmitHashPolicyMismatch`、`SlaveCountMismatch`(slave 数不足)、`MiimonDisabled`(miimon=0,未开链路检测)、`MiimonMismatch`、`UpdelayZero`/`UpdelayMismatch`、`DowndelayTooSmall`/`DowndelayMismatch`、`ActiveSlaveFlapping`(active slave 频繁切换)、`LinkFailureGrowing`(某 slave link failure 计数增长)。

**3. 常见根因**

- Bond 配置与 spec 不符(MTU、hash policy、miimon/updelay/downdelay) → 各类 `*Mismatch` / `*Disabled` / `*Zero`。
- 某个或多个 slave 掉链路 → `BondDown`、`SlaveCountMismatch`、`LinkFailureGrowing`。
- 物理层不稳导致主备频繁切换 → `ActiveSlaveFlapping`(此时优先查 L1 物理层)。

**4. 确认命令(只读)**

```bash
cat /proc/net/bonding/<bond>                      # MII Status / slave / active slave / link failure count
ip link show <bond>                               # MTU
cat /sys/class/net/<bond>/bonding/xmit_hash_policy
cat /sys/class/net/<bond>/bonding/miimon
cat /sys/class/net/<bond>/bonding/updelay
cat /sys/class/net/<bond>/bonding/downdelay
```

**5. 处置**

- 节点侧可修(多为配置纠偏):参数类不符 → 按 `/etc/netplan` 或 `network-scripts` 改回 spec 值(miimon>0、合理的 up/downdelay);`SlaveCountMismatch`/`BondDown` → 先按 L1 修复掉链路的 slave。
- `ActiveSlaveFlapping`/`LinkFailureGrowing` 频繁 → 根因常在物理层(线缆/口),交叉参考 L1 与 transceiver.md。

**6. 级别与升级** — Info(提示性,不 cordon)。配置类自行纠偏;若 bond 整体 down 影响业务连通,再评估人工摘节点。

---

## L3(LACP) — 链路聚合协商层 (Info)

**1. 是什么** — 针对 802.3ad(LACP)模式的 bond,校验与对端交换机的 LACP 协商是否正常:是否有有效 Active Aggregator、Partner MAC 是否有效、各 slave 的 Aggregator/Actor/Partner key 是否一致、lacp_rate 是否符合 spec。

**2. 典型输出**（`checker=L3(LACP)`,ErrorName 示意)

```
checker=L3(LACP) component=ethernet ErrorName=ActiveAggregatorMissing
Bond bond0 configured as 802.3ad mode but no valid Active Aggregator found. Command: cat /proc/net/bonding/bond0, peer switch might not have LACP configured or link is abnormal.
```

本层可能的 ErrorName:`NoBondInterface`、`ActiveAggregatorMissing`(802.3ad 模式但无有效 Active Aggregator)、`PartnerMacInvalid`(Partner MAC 全零,对端未回 LACP)、`AggregatorMismatch`(slave 的 Aggregator ID 与全局不符,无法入组)、`ActorKeyMismatch`、`PartnerKeyMismatch`(对端 LACP 协商异常)、`LACPRateMismatch`。

**3. 常见根因**

- 对端交换机未配 LACP / Eth-Trunk,或链路异常 → `ActiveAggregatorMissing`、`PartnerMacInvalid`。
- 交换机侧聚合组配置与本端不一致 → 各类 key/aggregator mismatch。
- lacp_rate(fast/slow)两端不一致 → `LACPRateMismatch`。

**4. 确认命令(只读)**

```bash
cat /proc/net/bonding/<bond>                    # Active Aggregator / Partner Mac / 各 slave key
cat /sys/class/net/<bond>/bonding/lacp_rate     # fast/slow
cat /sys/class/net/<bond>/bonding/mode          # 确认是否 802.3ad
```

**5. 处置**

- 节点侧可修:lacp_rate 不符 → 按 spec 改本端。
- 需联动交换机(升级网络团队):`ActiveAggregatorMissing`/`PartnerMacInvalid`/各 key mismatch 通常是对端交换机 LACP/Eth-Trunk 配置问题,需与网络团队同时排查两端聚合配置。

**6. 级别与升级** — Info(提示性,不 cordon)。LACP 协商问题多需交换机侧配合;影响业务多路径带宽时联系网络团队处理。

---

## L4(ARP) — 邻居可达性层 (Info)

**1. 是什么** — 校验 L2/L3 邻居可达性:ARP 邻居表是否有 FAILED/INCOMPLETE 条目(L2 MAC 解析失败),以及网关是否可达。

**2. 典型输出**（`checker=L4(ARP)`,ErrorName 示意)

```
checker=L4(ARP) component=ethernet ErrorName=ARPFailed
FAILED/INCOMPLETE entries found in ARP neighbor table. Command: ip neigh show, L2 MAC resolution failed for some neighbors.
```

本层可能的 ErrorName:`ARPFailed`(ARP 邻居表有 FAILED/INCOMPLETE 条目)、`GatewayUnreachable`(网关不可达)。

**3. 常见根因**

- VLAN ID 配置本端/交换机不一致,或 L2 转发异常 → `ARPFailed`。
- 网关地址配置错误、网关侧不通、或路由/VLAN 问题 → `GatewayUnreachable`。

**4. 确认命令(只读)**

```bash
ip neigh show                       # 看 FAILED/INCOMPLETE
ip route show                       # 网关配置
arping -I <bond> <gw>               # 测试 L2 可达(只读探测)
tcpdump -i <bond> arp -c 20         # 观察 ARP 交互
```

**5. 处置**

- 节点侧可修:核对本端 VLAN ID、网关地址与路由配置。
- 需联动交换机(升级网络团队):VLAN/L2 转发或网关侧问题,与网络团队核对交换机 VLAN 与网关状态。

**6. 级别与升级** — Info(提示性,不 cordon)。可达性问题若导致业务网不通,联系网络团队处理。

---

## L5(Routing) — 路由与反向路径层 (Info)

**1. 是什么** — 校验系统路由是否正确指向业务 bond,以及 rp_filter(反向路径过滤)配置是否会导致业务流量丢包。

**2. 典型输出**（`checker=L5(Routing)`,ErrorName 示意)

```
checker=L5(Routing) component=ethernet ErrorName=RPFilterEnabled
System enabled rp_filter (all=1). Command: sysctl -n net.ipv4.conf.all.rp_filter, Expected: 0 or 2, Actual: 1.
```

本层可能的 ErrorName:`DirectRouteMismatch`(默认路由未直接指向目标 bond,业务流量可能不走 bond)、`RPFilterEnabled`(`all` 或某 bond 的 `rp_filter=1`,严格反向路径过滤会误丢多路径/策略路由的包)。

**3. 常见根因**

- 默认路由指向了非业务网卡 → `DirectRouteMismatch`。
- 内核 `rp_filter` 设为 1(严格模式),在多网卡/策略路由场景下误丢包 → `RPFilterEnabled`。

**4. 确认命令(只读)**

```bash
ip route show default                       # 默认路由指向
sysctl -n net.ipv4.conf.all.rp_filter       # 期望 0 或 2
sysctl -n net.ipv4.conf.<bond>.rp_filter
ip rule show                                # 策略路由
```

**5. 处置**

- 节点侧可修:`DirectRouteMismatch` → 核对默认路由/策略路由,使业务流量走 bond;`RPFilterEnabled` → 若发生丢包,将相关 `rp_filter` 设为 0 或 2(松散模式)。
- 修改后用 `ip route`/`ip rule` 复核路由匹配。

**6. 级别与升级** — Info(提示性,不 cordon)。路由/rp_filter 属节点侧配置纠偏,通常无需摘节点;影响业务连通时优先按上面处置。
