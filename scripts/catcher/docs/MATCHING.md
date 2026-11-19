# catcher 型号匹配规则（matching spec）

这份文档把 catcher「硬件 → 规范键 → 机型ID」的**完整匹配规则**讲清楚，方便随时查阅、
核对、扩充。规则按代码实现如实描述：

- 数据（命名规范表）在 [`lib/catalog.sh`](../lib/catalog.sh)
- 硬件探测 → 规范键在 [`lib/detect.sh`](../lib/detect.sh)
- 规范键 → ID、ID → 配置说明在 [`lib/idgen.sh`](../lib/idgen.sh)

## 0. 两条路径

| 模式 | 入口 | 数据来源 | 权威性 |
|------|------|----------|--------|
| **detect / id / json** | 扫描本机 | 物理实测（驱动/固件/sysfs） | 权威：真实区分 以太/IB、SSD/HDD、DDR 代际 |
| **describe `<机型ID>`** | 只给 ID | 规范表 + 启发式 | 推断：DDR 由 CPU 代际推断、IB 代际按速率默认 |

匹配的核心是一个**规范键（canonical key）**：硬件无关的归一化描述。detect 产出规范键，
idgen 用它在 catalog 里查 ID。键的格式必须和 catalog 里的写法完全一致才能命中。

---

## 1. 规范键格式（每部件）

| 部件 | 规范键格式 | 例子 | → ID |
|------|-----------|------|------|
| CPU  | `"<厂商> <SKU>\|<插槽数>"` | `Intel 6530\|2` | `I57` |
| GPU  | `"<token>\|<数量>"` | `RTX5090\|8` | `G90` |
| 内存 | `"<容量GB>\|<条数>"` | `64\|16` | `M63` |
| 网卡 | `"<速率>Gx<端口数>*<卡数>[+…]"`（Gbps，**升序**，同速率合并） | `25Gx2*1+200Gx1*2` | `N23` |
| 硬盘 | `"<label>*<n>[+…]"`（容量**升序**，同容量合并） | `960G*2+3.84T*2` | `S10` |

匹配是**精确查表**（关联数组直查）：键在表里就返回 ID，不在就返回空 → 该部件标「未匹配」。
没有模糊/就近匹配——所有"容差"都发生在**生成规范键之前**（见各部件规则）。

---

## 2. CPU

- 命令：`lscpu`（读 CPUID 自报型号 + `Socket(s)`）。
- 厂商：`Vendor ID` 含 `Intel`/`GenuineIntel` 或 `AMD`/`AuthenticAMD`。
- SKU 解析：
  - **Intel**：先去掉 `(R)`/`(TM)`/`@…`，再取第一个 `[0-9]{4}[A-Z]*\+?`。
    `XEON GOLD 6530`→`6530`；`PLATINUM 8480+`→`8480+`；`…6548Y+`→`6548Y+`。
  - **AMD**：取 `EPYC` 后面那个 token；取不到则退回 `[0-9]{2}[0-9A-Z]{2}[A-Z]?`。
    `EPYC 7713`→`7713`、`9575F`→`9575F`、`74F3`→`74F3`。
- 键：`"<厂商> <SKU>|<插槽数>"`，插槽数取不到默认 `1`。
- 解析不到 SKU → `CPU_KEY` 为空，打 note。

> CPU 代际还决定 **DDR 推断**（describe 模式用）：Cascade Lake/Ice Lake/AMD Rome·Milan = DDR4；
> Sapphire/Emerald/Granite Rapids、AMD Genoa·Turin = DDR5。见 `CPU_MEMGEN`。detect 模式用 dmidecode 实测覆盖。

---

## 3. GPU

- 命令：`nvidia-smi --query-gpu=name,memory.total`（NVML，不需要 CUDA 工具包）。
- **token** 由产品名 + 显存共同决定（`_gpu_token`），因为同系列不同变体是不同 ID：
  - 总线：产品名含 `PCIE` → `PCIE`，否则默认 `SXM`。
  - 显存阈值：
    - A100：`显存 ≥ 70000 MiB` → 80G，否则 40G（→ `A100-<bus>-80G/40G`）。
    - H20：`显存 ≥ 130000 MiB` → `H20-141G`，否则 `H20-96G`。
  - 固定映射：`RTX 5090`→`RTX5090`、`H100`→`H100-SXM-80G`（库里只有 SXM-80G）、
    `H200`→`H200-141G`、`A800`→`A800-SXM-80G`、`V100S`→`V100S-PCIE-32G`、
    `V100`→ PCIe 则 `V100-PCIE-16G` 否则 `V100-SXM2`、`2080Ti`、`L40S/L40/L20/A10` 等。
  - 认不出 → `RAW:<原名>`，该 GPU 标「不在机型库」。
- **数量**：统计所有 GPU 行数；型号不一致时打 `mixed GPU models` note、取第一种。
- 键：`"<token>|<数量>"`，例 `RTX5090|8` → `G90`。

---

## 4. 内存

- 命令：`dmidecode -t memory`（**需 root**；没有 root 只能从 `/proc/meminfo` 读总量，无法区分条数）。
- 只统计已插入的内存条（跳过 `No Module / Not Installed / Unknown`）。
- **容量取众数（dominant size）**：出现最多的单条容量；若有多种容量，打 `mixed DIMM sizes` note 并用众数。
- **条数**：已插入内存条总数。
- DDR 代际：从 dmidecode 的 `Type:` 实测（`DDR4`/`DDR5`）。
- 键：`"<容量GB>|<条数>"`，例 `64|16` → `M63`。

---

## 5. 网卡（以太网 + InfiniBand）

- 命令：`/sys/class/net/<i>/speed`（Mb/s，链路 down 时退回 `ethtool`）、
  `/sys/class/infiniband/.../ports/*/rate`（如 `200 Gb/sec (4X HDR)`）。只数有 PCI `device` 的物理口。
- **以太 vs IB 的判定**：
  - **detect（实测，权威）**：按物理 `link_layer` 分。**RoCE（link_layer=Ethernet）即使 100G 也算以太网**，不算 IB。
  - **describe（按速率启发式）**：速率 `1/10/25` 视为以太网；`≥100` 视为 IB（`_eth_speed`）。
- **合并成键**：写成 `速率Gx端口数*卡数+…`，严格按源表原文分段，**不自行改写**：
  - `400Gbps *8` → `400Gx8*1`（源表把 8 当成端口数）
  - `400Gbps物理卡(1口)*8`（实测） → `400Gx1*8`（1 口 × 8 张卡）
  - `100Gbps*2+10Gbps*2+10Gbps*2+10Gbps*2`（N41）→ 保留三段 10G，不合并
- **匹配按端口总数归一化**（`nic_canon`）：把每段的 `端口数×卡数` 相加得到
  `<速率>:<总端口数>`，例如 `400Gx1*8` 与 `400Gx8*1` 都归一为 `400:8`，
  `1Gx2*1+10Gx2*1+10Gx2*1+10Gx2*1` 归一为 `1:2+10:6`。因此键的**书写顺序/分段方式**
  不影响命中，只有每档速率的**总端口数**参与匹配。
- **配置说明**：`describe`/`detect` 直接输出 `NIC_DESC` 里源表原文（逐字保留）。
  `render_nic_desc` 仅在 `NIC_DESC` 缺失时兜底：以太按 端口/2 折算双口卡，IB 按端口数折算单口卡。
- **IB 代际解析顺序**（高速口 200G 可能是 HDR 或 NDR200，需消歧）：
  1. 实测 `IB_GEN`（来自 `rate` 里的 `HDR/NDR/…`）；
  2. 否则查 per-ID 覆盖 `NIC_IBGEN`（如 `N20:200=NDR`、`N23:200=HDR`）；
  3. 否则按速率默认 `_ibgen_default`：`100=EDR, 200=HDR, 400=NDR, 800=NDR`。
- 链路 down 的口：不计入，打 note。

---

## 6. 硬盘

- 命令：`lsblk -dnb -o NAME,SIZE,TYPE,ROTA,MODEL`。`ROTA`：0=SSD/NVMe，1=机械盘 HDD。
- 跳过 `loop/zram/sr/fd/dm-/rbd`，只看 `TYPE=disk`。
- **容量就近贴桶（关键容差）**：原始字节 → 十进制 GB（`bytes/1e9`）→ 贴到最近的营销桶
  （`DISK_BUCKETS`：480/800/900/960/1200/…/12000 GB）。**只有落在桶的 ±8% 内才算命中**，
  否则该盘标 `unmatched`、打 note。这样「960GB SSD 实报 ~894GiB」「3.84TB ~3576GiB」都能正确归桶。
- **媒介（SSD/HDD）**：detect 用 `ROTA` 实测；describe 模式用源表里显式标了 HDD 的层（`DISK_MEDIA_SPEC`），其余默认 SSD。
- **合并成键**：各容量层**按容量升序**、同容量累加，写成 `label*数量+…`，例 `960G*2+3.84T*2` → `S10`。
- **配置说明拆分**：最小容量层 = 系统盘（2~3 块时标「需支持 raid1」），其余 = 数据盘。
- **RAID1 掩盖兜底**（`guess_disk_raid`）：硬件 RAID 逻辑卷会把启动盘对显示成一个卷
  （如 RAID1 的 960G×2 只露一个 ~894GiB）。检测到 `Logical Volume/Virtual Disk/MR9/PERC/AVAGO` 时打 RAID note，
  并对每个「单块」层尝试翻倍成 `*2` 再查一次机型库（`960G*1+3.84T*2` → 命中 `S10`）。

---

## 7. 机型ID 拼接与反解析

- **拼接顺序固定**：CPU → GPU → 网卡 → 内存 → 硬盘，例 `I57G90N13M63S10`。
- **反解析 `split_id`**：每个 token = **一个字母 + 两个 `[0-9A-Z]`**（CPU 可以 hex 字母结尾，如 `I3A`）。
  从左到右按首字母归类：`I/A`→CPU、`G`→GPU、`N`→网卡、`M`→内存、`S`→硬盘。
- 任一部件未匹配机型库 → 黄色提示 `未匹配到机型库 … 请补充命名规范`，机型ID 标记为不完整。

---

## 8. 已知规范歧义（按源表如实保留，勿"修正"）

- `M60` 与 `M61` 同为 `64G*8`：**以 `M61` 为准**，`["64|8"]=M61`；`M60` 已完全移除
  （键映射与描述都不再存在，`describe ...M60...` 会提示未知代码）。
- `S25`（`1.2TB*1+3.84TB*1`）与 `S32`（`1.2T *1 +3.84T *1`）同为 1.2T*1+3.84T*1：
  规范键归 `S25`，**`S32` 未入库**（按维护要求省略）。
- 描述文本按工具既有风格书写（`*2` 而非 `* 2`），与规范表的空格差异不算不一致；
  GPU 描述保留 `PCIe/SXM/nvlink/显存` 区分，但用工具风格（`H200 *8` 而非 `H200-nvlink141G * 8`）。
- `S06` 是 `1.6TB*1`；`1.7T*1` 作为同容量档别名加入（两个桶距离过近，实测会落在
  `1700` 桶），反解析取 ASCII 序第一个键 → 固定显示 `1.6TB *1`。
- `N33` 源表直接写成规范键形式 `100Gx2*1+100Gx4*1+200Gx2*1+200Gx4*8`，按端口数展开是
  `100Gbps*6 + 200Gbps*34`（2+4 个 100G 口，2+4×8 个 200G 口）。
- `N31` 源表写作 `25Gbps *2 + 200G*2 + 400Gbps *8`，按端口数 2 解析。
- `G7B` = `RTX 3090 *4`（`RTX3090|4`）。
- `H100`/`A800` 库内只有 SXM-80G 变体。

---

## 9. 如何新增 / 修改一个机型

1. 在 [`lib/catalog.sh`](../lib/catalog.sh) 对应部件的 `*_ID` 加一行「规范键 → 新代码」，并在 `*_DESC` 加可读描述。
2. 确认 detect 能产出**完全一致**的规范键：
   - GPU 新变体可能要在 `detect.sh` 的 `_gpu_token` 加判定；
   - 新硬盘容量要加进 `DISK_LABEL` + `DISK_BUCKETS`；
   - 新内存代际要补 `CPU_MEMGEN`（describe 模式用）。
3. 用 `describe <新ID>` 反解析自检，再在目标机上 `sudo ./catcher.bash detect` 实测核对。

## 10. 任意机器上的快速自检

```bash
sudo ./catcher.bash detect        # 机型ID + 配置说明（内存检测需 root）
./catcher.bash describe <机型ID>   # 反解析核对，不读硬件、不需 root
```
若某部件打「未匹配」黄字，就按第 9 节把规范键补进 `catalog.sh`。
