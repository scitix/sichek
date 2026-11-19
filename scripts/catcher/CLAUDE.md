# catcher — project notes

机型ID 与实际配置检查对比工具。抓取物理机 CPU/内存/硬盘/GPU/网卡 实配，按命名规范生成
机型ID，并输出配置说明；也可把机型ID 反解析成配置。Companion to `uqsm5090`
(同样纯 bash + 标准工具，物理机上裸跑，最终打包上传 OSS)。

## 设计要点

- **数据与逻辑分离**：所有命名规范在 `lib/catalog.sh`（关联数组，易核对源表）；探测在
  `lib/detect.sh`；匹配/描述在 `lib/idgen.sh`；编排在 `catcher.bash`。
- **规范键 (canonical key)** 是探测与机型库之间的契约，detect 产出的键必须与 catalog 的键格式一致：
  - CPU `"<Vendor> <SKU>|<sockets>"`、GPU `"<token>|<count>"`、内存 `"<GB>|<count>"`、
    网卡 `"<spd>*<n>+..."`(Gbps 升序合并)、硬盘 `"<label>*<n>+..."`(容量升序合并)。
- **两种描述路径**：
  - `detect`/`json`：`render_detected_config`，**物理实测权威** —— 真实 DDR、SSD/HDD(ROTA)、
    以太网 vs IB(物理口类型，不靠速率猜)。
  - `describe <ID>`：只有 ID，启发式 —— DDR 由 `CPU_MEMGEN` 按 CPU 代际推断；IB 代际按
    `NIC_IBGEN` 覆盖(N20→NDR, N23→HDR) 否则按速率默认；硬盘最小层=系统盘(raid1 备注)、其余=数据盘。

## 关键硬件识别命令（写给会读硬件的人）

- GPU：`nvidia-smi` 走 NVML/驱动，无需 CUDA toolkit（正合 5090 这类机器）。产品名编码
  PCIe/SXM 与显存，用来区分 A100-PCIe-40G vs A100-SXM-80G 等不同 ID。
- CPU：`lscpu` 的 `Model name` 取 SKU（Intel 取 tier 后的 `[0-9]{4}[A-Z]*\+?`，注意 `6530` vs
  `6530P` 差一个字母；AMD 取 `EPYC` 后一个 token，含 `74F3`/`9575F` 这类）；`Socket(s)` 是物理颗数。
- 内存：`dmidecode -t memory`（需 root，读固件 SMBIOS），统计每根容量+条数+DDR 代；`/proc/meminfo`
  只能给总量，区分不了 16×64G 与 32×32G。
- 硬盘：`lsblk -dnb` 读字节(十进制)，`ROTA` 区分 SSD/HDD；营销容量用 1000 进制，TiB 显示偏小
  (960GB≈894GiB, 3.84TB≈3576GiB)，故按字节贴近 `DISK_BUCKETS`(±8%)。硬件 RAID 逻辑盘会掩盖物理盘数。
- 网卡：只数有 `/sys/class/net/<i>/device` 的物理口；速率取 `.../speed`(Mb/s)；bond 不重复计数；
  IB 速率+代际(HDR/NDR) 取 `/sys/class/infiniband/<dev>/ports/*/rate`(如 `200 Gb/sec (4X HDR)`)，
  RoCE(link_layer=Ethernet) 计为以太网。规范键格式 `<spd>Gx<ports>*<cards>`，严格照源表原文推导；
  匹配走 `nic_canon`（按速率求端口总数）后再比，`400Gx1*8` 与 `400Gx8*1` 等价；
  描述直接输出 `NIC_DESC`（源表原文），`render_nic_desc` 只作兜底。

## 源表里的歧义（如实保留，勿"修正"）

- `M60==M61==64G*8`，**以 M61 为准，M60 已删除**；`S06=1.6TB*1`（另留 `1.7T*1` 别名）；
  `N41` 源表把 10G 拆成三段并列，`NIC_DESC` 照原文输出。改动这些前先与机型库 owner 确认。

## 测试基准

- 4 个示例 ID 必须复现：`I31G82N23M65S03 / I31G82N20M65S03 / I31N23M62S28 / I31N23M65S03`
  （`./catcher.bash describe <ID>`）。HDR/NDR、DDR4、系统盘/数据盘拆分、S28 的 2.4T=HDD 都要对。
- 本机(shmaas-g90-036, 8×5090) `detect` 应得 `I57 G90 N13`，内存/硬盘随 RAID 提示。

## 发布到 OSS

与 uqsm5090 相同：`ossctl cp catcher.tar.gz oss/scitix-release/catcher.tar.gz`
（凭据在发布机 `~/.ossctl/config.json`，**不入库**；公共读，wget 即可下载）。
