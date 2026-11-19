# server_model_id 上报设计（集成硬件团队 catcher）

日期：2026-11-13　分支：`feat/server-model-id`（基于 main）

## 1. 背景与目标

硬件团队提供 catcher 工具（`https://oss-cn-shanghai.siflow.cn/scitix-release/catcher.tar.gz`），
在物理机上采集 CPU / GPU / 网卡 / 内存 / 硬盘，按命名规范表生成机型ID（如 `I57G90N13M63S10`）。

目标：sichek daemon 定期在每台节点上运行 catcher，把机型ID 及其明细写进 snapshot、
随现有 reporter 上报 collector，并导出 Prometheus 指标。

- 第一期只上报，不做判定（无 checker、不进 K8s annotation、不打 node label）。
- 定位为**节点元信息**（与 snapshot 的 `mgmt_ip` / `boot_time` 同类），放在 `service/` 层，不新增 component。

## 2. catcher 关键事实（v: sha256 021a25d6…，2026-11-13 拉取）

- 纯 bash（~1000 行），只读，不联网。依赖 `lscpu`、`nvidia-smi`、`dmidecode`(需 root)、`lsblk`、
  `ethtool`、可选 `storcli/storcli2`、`/sys/class/{net,infiniband}`。
- `catcher.bash json` **不输出到 stdout**，而是写 `<脚本目录>/result/catcher_result.json`：
  `{model_id, components:{cpu,gpu,nic,mem,disk:{code,detected,key}}, config}`。
- 任一部件未匹配命名表时该段 code 为空、ID 缺段，脚本退出码仍为 0。
- 没有 GPU 的机器 GPU 段为空属正常（catcher 自身也不计为缺失）。

## 3. 打包

- catcher 源码原样放入 `scripts/catcher/`，附 `scripts/catcher/VERSION`（tar 包 sha256 + 拉取日期）。
- goreleaser 已把 `./scripts` 装到 `/var/sichek/scripts`，deb 与镜像无需改打包配置 →
  安装路径 `/var/sichek/scripts/catcher/catcher.bash`。
- `docker/Dockerfile*` 运行时镜像补装 `dmidecode`、`ethtool`（Ubuntu: apt；centos8: yum）。
  `storcli` 为厂商工具不入镜像，RAID 机器硬盘段可能缺失，通过 `missing` 如实体现。
- 升级 catcher：替换 `scripts/catcher/` 并更新 `VERSION`，随 sichek 发版。

## 4. 执行（`service/servermodel.go`）

### 4.1 调度

- daemon `Run()` 中启动一个 goroutine（受 `d.ctx` 控制）。
- **启动后等待 5 分钟执行首次**（错开启动期各组件集中 NVML 采集，规避 DCGM profiling 被顶掉的已知竞态），
  **之后每 1 小时执行一次**。
- 配置项（user config，缺省值如下，缺省/解析失败时使用默认值）：

```yaml
server_model:
  enable: true
  script_path: "/var/sichek/scripts/catcher/catcher.bash"
  initial_delay: 5m
  interval: 1h
  timeout: 120s
```

### 4.2 单次运行

1. 把 `script_path` 所在目录复制到 `os.MkdirTemp` 临时目录（catcher 会写自身目录下 `result/`，安装目录可能只读）。
2. `bash <tmp>/catcher.bash json`，`SysProcAttr.Setpgid=true`；超时后 `kill(-pgid, SIGKILL)` 杀整个进程组
   （普通 ctx 超时只杀直接子进程，孙进程握管道会让等待一直阻塞——仓库已知问题）。
3. 读 `<tmp>/result/catcher_result.json` 并解析。
4. `defer os.RemoveAll(tmp)`。

### 4.3 结果计算

- `server_model_id` = catcher 的 `model_id`。
- `missing`：cpu / nic / mem / disk 的 `code` 为空即缺失；gpu 仅在 `code` 为空且 `detected` 或 `key` 非空时算缺失。
- `complete` = `model_id` 非空且 `missing` 为空。
- `catcher_version` 取 `VERSION` 文件内容（读不到为空串）。

## 5. 数据结构

`Snapshot` 新增两个顶层字段：

```json
"server_model_id": "I57G90N13M63S10",
"server_model": {
  "complete": true,
  "missing": [],
  "components": {
    "cpu":  {"code": "I57", "detected": "INTEL(R) XEON(R) GOLD 6530 (2 socket)", "key": "Intel 6530|2"},
    "gpu":  {"code": "G90", "detected": "NVIDIA GeForce RTX 5090 *8", "key": "RTX5090|8"},
    "nic":  {"code": "N13", "detected": "100Gbps*2", "key": "100Gx2*1"},
    "mem":  {"code": "M63", "detected": "DDR5 64G *16", "key": "64|16"},
    "disk": {"code": "S10", "detected": "SSD 960G*2 + SSD 3.84T*2", "key": "960G*2+3.84T*2"}
  },
  "config": "CPU: Intel 6530 *2\n内存: DDR5 64G *16\n...",
  "catcher_version": "sha256:021a25d6... 2026-11-13",
  "collected_at": "2026-11-13T10:05:00Z",
  "error": ""
}
```

- 首次运行前两个字段均缺省（`omitempty`）。
- `SnapshotManager.SetServerModel(id string, sm *ServerModel)` 加锁写入，随下次 persist 落盘。

## 6. 上报

- **snapshot / collector**：reporter 每 60s 原样 POST snapshot，`server_model_id` 与明细随之上报，reporter 不改。
  collector/Doris 侧解析新字段不在本仓库范围。
- **Prometheus**：`sichek_server_model_info{server_model_id="<ID>", complete="true|false", node="<node>"} 1`。
  ID 或 complete 变化时先删除旧 label 组合再写新值，保证每节点仅一条。
  运行失败时保留上次的指标不变。

## 7. 错误处理

| 情况 | 行为 |
|------|------|
| `enable=false` | 不启动 goroutine，snapshot 无这两个字段 |
| 脚本不存在 / 复制失败 / 超时 / 非 0 退出 / 结果文件缺失 / JSON 解析失败 | `error` 写入原因，`collected_at` 更新；**保留上次成功的 `server_model_id` 与明细**；warn 日志；不影响任何组件 |
| ID 缺段（如无 storcli 时 disk 缺失） | 正常上报，`complete=false`，`missing` 列出缺失部件 |
| catcher 成功但 `model_id` 为空 | 视为成功一次（`error` 为空），`complete=false` |

## 8. 测试

- 单元测试（表驱动，testify，`t.TempDir()`）：
  - 解析：完整结果、缺段结果、无 GPU 机器、损坏 JSON。
  - `missing`/`complete` 计算规则。
  - 失败时保留上次成功值、仅更新 `error`。
  - 用假脚本（写固定 JSON / sleep 后台孙进程）验证正常执行与超时杀进程组。
  - Prometheus：ID 变化后旧 label 组合被删除。
- 现场回归：B300 与 H 系列各一台，核对 snapshot 字段、`/metrics`、collector 收到报文，
  并与 `catcher.bash detect` 人工输出对照；测试 daemon 须隔离 snapshot/reporter 配置。

## 9. 不在本期范围

- collector / Doris / argus 侧字段解析与展示。
- 与 CMDB 登记机型比对的 checker（如需，可另加 checker 读取本结果）。
- K8s node label。
- 运行时从 OSS 拉取 catcher。
