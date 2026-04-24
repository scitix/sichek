# sichek-collector 设计文档

**日期**：2026-04-23
**作者**：lzi
**状态**：Draft — 待评审

---

## 1. 背景与目标

sichek daemon 在每个 GPU 节点的宿主机 `/var/sichek/data/snapshot.json` 写入该节点所有组件的健康快照。现状下，snapshot 只存在节点本地，跨集群分析服务无法获取。

本设计新增：

1. **节点侧 Reporter 模块**（改动在 `scitix/sichek` 仓库）：作为 daemon 内部 goroutine 周期性 POST snapshot 到集群内 collector。
2. **sichek-collector 独立应用**（**新建独立仓库**）：集群内单点 HTTP 服务，接收 POST，按节点名落盘，保留"每节点最新一份"。
3. **集群外分析服务**通过 SSH/rsync 捞文件、或可选 GET 接口统一拉取。

### 业务目标

- **集群态势可视化**：分析服务聚合统计（例如今天全集群有多少节点 GPU 温度异常）
- **长期归档 / 审计**：由外部分析服务按其自身策略长期保存
- **数据源供给**：作为已有分析服务的数据源

### 非目标

- 不在 collector 内做归档（只保留最新，不做日期切片）
- 不在 collector 内做聚合分析或查询索引
- 不做 HA（单副本足够，丢一次上报下一轮补）
- 不做节点侧本地历史缓冲（collector 只要最新）

---

## 2. 架构总览

```
┌────────────────────────────────────────────────────────────────────┐
│  每个 GPU 节点 (existing sichek DaemonSet, 宿主机 systemd daemon)  │
│                                                                    │
│  sichek daemon                                                     │
│  ├── Checker Loop → SnapshotManager.Update() → snapshot.json       │
│  └── [NEW] Reporter goroutine                                      │
│        └── 周期读 snapshot.json → gzip → POST 到 collector         │
└──────────────────────────────────┬─────────────────────────────────┘
                                   │ HTTP POST (cluster internal)
                                   ↓
┌────────────────────────────────────────────────────────────────────┐
│  sichek-collector (NEW, 独立仓库) — 单副本 Deployment + PVC         │
│                                                                    │
│  ┌──────────────────────────────────────────┐                      │
│  │ HTTP Server (net/http)                   │                      │
│  │   POST /api/v1/snapshots  → 覆写         │                      │
│  │   GET  /api/v1/snapshots  → bulk NDJSON  │ (可选,对外)           │
│  │   GET  /healthz                          │                      │
│  └────────────────┬─────────────────────────┘                      │
│                   ↓                                                │
│  ┌──────────────────────────────────────────┐                      │
│  │ PVC /data/<node>.json                    │                      │
│  │   (atomic write: tmp + rename, 最新一份)  │                      │
│  └──────────────────────────────────────────┘                      │
│                                                                    │
│  Service: ClusterIP: 38080 (POST 内部)                             │
│  Service: NodePort / Ingress (GET 外部,可选)                       │
└────────────────────────────────────────────────────────────────────┘
                                   ↑
                                   │ 分钟级 rsync/SCP 捞文件 或 HTTP GET
                                   │
                           ┌───────┴────────┐
                           │ 分析服务        │
                           │ (集群外)        │
                           └────────────────┘
```

### 核心原则

- **Collector 只存最新**：每节点一个文件，新上报覆写，空间恒定（1000 节点 × 90KB ≈ 90MB 常驻）。
- **Collector 对 snapshot 内容无认知**：存原始字节，节点名来自 HTTP header 的权威来源；snapshot 格式未来演化不影响 collector。
- **Reporter 作为 daemon 模块**：复用现有 hostname、配置加载、日志、SnapshotManager，最小改动。
- **职责最小化**：collector 只做"接收 + 写文件 + 可选聚合读"，不做归档、不做 metrics、不做认证（除非对外 GET）。

---

## 3. 节点侧 Reporter（改动在 sichek 仓库）

### 3.1 新增文件

```
service/reporter.go        # Reporter struct + 周期上报循环
service/reporter_test.go   # 单元测试
```

### 3.2 Reporter 职责

1. 按配置间隔读取 `snapshot.json` 字节流
2. gzip 压缩（如启用）
3. HTTP POST 到 collector endpoint
   - Header: `X-Sichek-Node: <nodename>`, `Content-Type: application/json`, `Content-Encoding: gzip`
4. 失败重试（指数退避，上限 `retry_max` 次），仍失败则跳过本轮
5. 总开关关闭时完全 no-op（不读文件、不建连接）

### 3.3 节点名来源

优先级从高到低：
1. 环境变量 `NODE_NAME`（DaemonSet 通过 `fieldRef: spec.nodeName` 注入）
2. `os.Hostname()`

使用 DaemonSet env 最可靠，避免 hostname ≠ K8s node 名的边界情况。

### 3.4 daemon 集成

- 在 `daemon.go` 启动流程：若 `reporter.enable == true`，启动 `go reporter.Run(ctx)`
- 复用 `SnapshotManager` 的 `path` 字段，读取同一份 snapshot 文件
- daemon 退出时通过 `context.Cancel` 优雅关闭 reporter goroutine

### 3.5 配置段（加入 `config/default_user_config.yaml`）

```yaml
reporter:
  enable: false                                              # 总开关,默认关闭;部署时显式打开
  endpoint: "http://sichek-collector.monitoring.svc:38080/api/v1/snapshots"
  interval: 60s                                              # 上报周期
  timeout: 30s                                               # 单次 POST 超时
  retry_max: 3                                               # 失败重试次数
  gzip: true                                                 # 请求体 gzip 压缩
```

**为什么默认 `enable: false`**：
新功能先发布再启用，避免 sichek 升级时对未部署 collector 的集群发起请求。运维流程：先部署 collector → 改 ConfigMap `enable: true` → sichek daemon 重启生效。

### 3.6 失败与降级

| 场景 | 行为 |
|---|---|
| snapshot.json 不存在 | 跳过本轮，log warn |
| snapshot.json 读失败 (IO error) | 跳过本轮，log warn |
| collector 连接失败 / 超时 | 指数退避重试 `retry_max` 次；仍失败跳过本轮 |
| collector 返回 4xx | 不重试（配置/鉴权问题），log error |
| collector 返回 5xx | 重试 `retry_max` 次 |
| POST body 超过 collector `MAX_BODY_SIZE` | 收到 413，不重试，log error |
| daemon 退出 | ctx cancel，Reporter goroutine 退出 |

### 3.7 不做的事

- 不做本地历史缓冲（collector 只要最新，漏一轮下一轮补）
- 不做配置热加载（改配置需重启 daemon，与现有 daemon 配置模型一致）
- 不做事件驱动（定期轮询简单够用，1000 节点 60s 周期下 ~16 QPS 无压力）

---

## 4. Collector 应用（新仓库 `sichek-collector`）

### 4.1 仓库结构

```
sichek-collector/
├── cmd/
│   └── sichek-collector/main.go     # 入口: 读 env/flag, 起 HTTP server
├── internal/
│   ├── server/                      # HTTP handlers
│   │   ├── server.go                # router, middleware
│   │   ├── snapshot.go              # POST/GET /api/v1/snapshots
│   │   └── health.go                # /healthz
│   └── store/                       # 存储抽象
│       ├── store.go                 # interface
│       └── fs.go                    # filesystem 实现 (atomic write)
├── deploy/                          # K8s manifests
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── pvc.yaml
│   └── ingress.yaml                 # 可选 (GET 对外)
├── Dockerfile
├── Makefile
├── go.mod                           # 独立 module, 不 import sichek
└── README.md
```

**独立仓库**：不复用 sichek 的 snapshot struct，体现解耦；collector 对内容无感知。

### 4.2 HTTP 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/v1/snapshots` | 节点上报;header `X-Sichek-Node: <hostname>` 必填;body 写入 `/data/<node>.json`（tmp + rename 原子覆写）;支持 `Content-Encoding: gzip`（自动解压） |
| `GET` | `/api/v1/snapshots` | 可选(`ENABLE_READ_API=false` 默认关);NDJSON 流式返回所有节点,每行 `{"node":"<name>","snapshot":{...}}` |
| `GET` | `/healthz` | 返回 200 |

### 4.3 写入语义

- 路径：`{DATA_DIR}/{sanitized_node}.json`，node 名过滤 `/`、`..` 等危险字符（返回 400）
- 原子性：`write tmp file` → `os.Rename`（同 FS 上 rename 原子）
- 无锁：不同节点写不同文件，同节点并发写靠 rename 原子性（后到的赢，符合"only latest"语义）
- Body 直接写入（如果请求带 `Content-Encoding: gzip`，先解压再写）
- **不解析 body**：collector 不 unmarshal JSON；即使误发非 JSON 也接受

### 4.4 GET bulk 接口（可选）

- 默认**关闭**（`ENABLE_READ_API=false`）
- 启用时返回 NDJSON 流（不是 JSON 数组），每行一个对象：
  ```
  {"node":"gpu-node-001","snapshot":{...}}
  {"node":"gpu-node-002","snapshot":{...}}
  ```
- 实现：遍历 `DATA_DIR` 下所有 `*.json`，用 `json.Encoder` 逐行写 response；内存占用 O(单文件大小) 而非 O(总体积)
- node 名来自**文件名**（即 POST 时的 `X-Sichek-Node`），不依赖 snapshot 内部字段
- 启用 + 对外暴露时通过 Bearer token 鉴权（`READ_TOKEN` env；空表示不鉴权）

### 4.5 配置（env / flag）

| 名称 | 默认 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `:38080` | HTTP 监听地址 |
| `DATA_DIR` | `/data` | 存储目录（挂 PVC） |
| `ENABLE_READ_API` | `false` | 是否启用 `GET /api/v1/snapshots` |
| `MAX_BODY_SIZE` | `10MB` | POST body 上限 |
| `READ_TOKEN` | `""` | 若启用 read API 且对外暴露,要求 `Authorization: Bearer <token>`;空表示不鉴权 |
| `LOG_LEVEL` | `info` | 日志级别 |

### 4.6 K8s 资源

- **Deployment**：1 副本，`strategy.type: Recreate`（同 PVC 互斥）；资源 limits `128Mi / 100m` 足够
- **Service (ClusterIP)**：port `38080`，节点上报走此
- **Service (NodePort / Ingress)**：仅在 `ENABLE_READ_API=true` 且外部要走 HTTP 取数时启用
- **PVC**：2GB, ReadWriteOnce（1000 节点 × 90KB = 90MB，留足余量）

### 4.7 错误处理

| 场景 | 响应 |
|---|---|
| 缺 `X-Sichek-Node` header | 400 Bad Request |
| node 名含 `/`、`..` 等危险字符 | 400 Bad Request（拒绝路径穿越） |
| body 超过 `MAX_BODY_SIZE` | 413 Payload Too Large |
| gzip 解码失败 | 400 Bad Request |
| 磁盘写失败 | 500 Internal Server Error；tmp 文件清理；旧文件保持不变 |
| DATA_DIR 不可写（启动时检查） | 启动失败，exit 非 0 |

### 4.8 不做的事（本设计范围外）

- 不做 Prometheus `/metrics` 端点（将来需要节点上报健康度，可在 sichek-exporter 里加 `sichek_reporter_last_push_timestamp` 指标代替）
- 不做数据库/索引（文件足够，分析服务自己建索引）
- 不做日期分区/归档（只要最新）
- 不做 HA/多副本（丢一次上报下一轮补）

---

## 5. 数据流

### 5.1 节点上报（写路径）

```
sichek daemon checker ──┐
                        ├──→ SnapshotManager.Update() ──→ /var/sichek/data/snapshot.json
    (每组件 Check 时)    ┘

Reporter ticker (每 60s) ──┐
                           ├──→ 读 snapshot.json
                           ├──→ gzip 压缩
                           ├──→ POST http://sichek-collector.monitoring.svc:38080/api/v1/snapshots
                           │       Header: X-Sichek-Node: <NODE_NAME>
                           │       Body: <gzipped snapshot bytes>
                           │
collector.snapshot handler ┘
    ├── 校验 header、node 名
    ├── 如果带 Content-Encoding: gzip,解压
    ├── 检查 MAX_BODY_SIZE
    ├── 写 /data/<node>.json.tmp
    └── rename → /data/<node>.json
```

### 5.2 外部拉取（读路径，两种方式）

**方式一：SSH/rsync 捞文件（默认模式，不启用 GET 接口）**
```
分析服务 (集群外) ──→ SSH 到 collector pod 所在节点
                  ──→ rsync /var/lib/kubelet/pods/<uid>/volumes/.../data/*.json
                      或 kubectl exec <pod> -- tar cf - /data | tar xf -
```

**方式二：HTTP GET（启用 `ENABLE_READ_API=true`）**
```
分析服务 (集群外) ──→ GET https://<cluster-ingress>/api/v1/snapshots
                      Authorization: Bearer <READ_TOKEN>
                  ──→ NDJSON 流式解析
```

---

## 6. 容量与性能评估

基于实测 snapshot.json 大小 **~90KB** 与目标规模 **1000 节点**：

| 指标 | 数值 |
|---|---|
| Collector 常驻磁盘 | 1000 × 90KB = **~90MB** |
| 入方向带宽（60s 周期） | 1000 × 90KB / 60s = **1.5 MB/s** |
| 入方向 QPS | 1000 / 60s ≈ **17 QPS** |
| Bulk GET 响应体积 | **~90MB**（NDJSON 流式，内存 O(单文件)） |
| 磁盘写 IO | **1.5 MB/s**（可忽略） |
| PVC 规格 | **2GB** 足够 |

所有数值都在单副本轻量 Pod 能力内，无需分片/分库/压缩即可满足（gzip 仍然默认打开，留未来增长余地）。

---

## 7. 测试计划

### 7.1 sichek 仓库（`service/reporter_test.go`）

- `TestReporter_Push_Success`：启动 `httptest.Server`，触发一次上报，断言 server 收到期望 body + header
- `TestReporter_Push_RetryOn5xx`：server 先 500 后 200，断言 client 总发 2 次
- `TestReporter_Push_NoRetryOn4xx`：server 400，断言只 1 次
- `TestReporter_DisabledNoOp`：`enable: false` 时 server 永不被调用
- `TestReporter_GzipEncoding`：断言请求 header `Content-Encoding: gzip`，body 可 gunzip 还原
- `TestReporter_MissingSnapshotFile`：文件不存在时不崩溃，下一轮继续
- `TestReporter_NodeNameFromEnv`：`NODE_NAME` 优先于 hostname

### 7.2 sichek-collector 仓库

**`internal/store/fs_test.go`**
- `TestFS_Write_Atomic`：写入后读回字节一致
- `TestFS_Write_Overwrite`：同节点两次写，第二次覆盖
- `TestFS_Write_InvalidNodeName`：`/`、`..`、空字符串被拒
- `TestFS_List_Empty`：空目录返回空列表

**`internal/server/snapshot_test.go`**
- `TestHandler_Post_OK`：200 + 文件落盘内容一致
- `TestHandler_Post_MissingNodeHeader`：400
- `TestHandler_Post_OversizedBody`：413
- `TestHandler_Post_GzipBody`：`Content-Encoding: gzip` 的请求正确解压后落盘
- `TestHandler_Get_Disabled`：`ENABLE_READ_API=false` 时 404
- `TestHandler_Get_Empty`：空目录返回 200 + 空 body（NDJSON 0 行）
- `TestHandler_Get_Stream`：多文件时按 NDJSON 格式流式返回，每行一条
- `TestHandler_Get_TokenAuth`：`READ_TOKEN` 非空时无 token 返回 401

**`cmd/sichek-collector/integration_test.go`**
- 起真实进程（`-LISTEN_ADDR=:0` 分配端口），端到端 POST + GET，用 `t.TempDir()` 做数据目录

### 7.3 CI

- sichek 仓库：现有 CI 流程不变，新测试自动进入
- sichek-collector 仓库：新建 GitHub Actions（lint + `go test ./...` + 镜像构建）

---

## 8. 部署与上线步骤

1. **阶段一：Collector 先行**
   - 在新仓库完成 collector 实现 + 测试 + 镜像发布
   - 在目标集群应用 `deploy/*.yaml`（Deployment + Service + PVC）
   - 验证 `/healthz` 可达
2. **阶段二：sichek 版本发布**
   - sichek 仓库合入 reporter 模块
   - 发布新版镜像（reporter 默认 `enable: false`）
   - 所有集群升级 sichek（行为不变）
3. **阶段三：逐集群启用**
   - 在目标集群 ConfigMap `sichek-default-user-config` 中改 `reporter.enable: true`
   - 滚动重启 sichek DaemonSet
   - 通过 collector 日志观察上报节点数增长；正常应在一个 interval 内收到全部节点
4. **阶段四：外部分析服务接入**
   - SSH/rsync 模式：给分析服务配置到 collector pod 所在节点的拉取路径
   - 或启用 `ENABLE_READ_API=true` + Ingress + token，分析服务切到 HTTP 拉

---

## 9. 风险与取舍

| 风险 | 概率 | 缓解 |
|---|---|---|
| Collector 单点宕机 | 中 | 丢失期间上报，恢复后下一轮补；不影响 daemon 主循环 |
| PVC 节点故障导致数据丢失 | 低 | 当天丢失可接受（只是"最新"）；分析服务保留自己的历史 |
| Reporter 引入 daemon bug 影响健康检查主路径 | 低 | Reporter 与主 Checker 完全解耦的 goroutine，`recover` 兜底 |
| 未来 snapshot 体积膨胀 | 中 | gzip 默认开；实际要超 10MB 时再加流控和频率下调 |
| 节点名冲突（两个 hostname 相同的节点） | 低 | 使用 K8s `NODE_NAME` 注入，K8s 层面保证唯一 |

---

## 10. 附录

### 10.1 为什么不复用 sichek 的 snapshot struct

Collector 独立仓库、对 snapshot 内容无认知，这意味着：

- snapshot 字段新增/删除/重命名 → collector 零改动
- 分析服务的 schema 需求变化 → 只需 reporter/snapshot 格式调整
- 可独立发布和回滚

代价是 collector 代码里不会出现 `type Snapshot struct { ... }` 这种结构化定义，所有 body 按 `[]byte` 处理。90KB 的 JSON 不存在反序列化需求，这个代价为零。

### 10.2 为什么不用 CronJob / 事件驱动

- CronJob：到点起 1000 个 Pod 去读 hostPath，调度抖动、资源浪费、权限复杂
- 事件驱动（snapshot 更新即推）：需要 SnapshotManager 增加订阅机制，代码更复杂；实际"更新"在各组件 Check 后几乎连续发生，和定时轮询等效
- 定时轮询是最简方案，够用

### 10.3 与现有 `sichek-exporter` 的关系

- `sichek-exporter` 提供 Prometheus metrics（硬件健康指标），是**实时**可观测性
- `sichek-collector` 提供完整快照归档给分析服务做**聚合分析/历史比对**
- 两者互补，不重复：metrics 回答"现在怎么样"，snapshot 回答"完整状态是什么"
