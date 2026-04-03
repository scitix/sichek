# Sichek K8s 部署架构详解

> 基于 `k8s/devops_deploy.yaml` 分析

---

## 一、部署模型概览

Sichek 采用 **DaemonSet + 宿主机安装** 模式：sichek daemon 实际运行在宿主机上（由 systemd 管理），DaemonSet 中的容器负责安装、保活和指标代理。

```
┌─────────────────────────────────────────────────────────────────┐
│  K8s DaemonSet Pod (每个 GPU 节点一个)                            │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐   │
│  │ sichek-init  │  │   sichek     │  │  sichek-exporter    │   │
│  │ (initContainer)│ │ (主容器)      │  │ (sidecar)           │   │
│  │              │  │              │  │                     │   │
│  │ 安装 sichek   │  │ 保活 daemon   │  │ socket→TCP 代理     │   │
│  │ 到宿主机      │  │ + 转发日志    │  │ Prometheus 抓取     │   │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬──────────┘   │
│         │ nsenter          │ nsenter              │              │
└─────────┼──────────────────┼──────────────────────┼──────────────┘
          ↓                  ↓                      ↓
┌─────────────────────────────────────────────────────────────────┐
│  宿主机                                                         │
│                                                                 │
│  systemd: sichek.service (daemon 模式)                           │
│    → 健康检查 → K8s annotation + metrics.sock + snapshot.json     │
│                                                                 │
│  /var/sichek/                                                   │
│  ├── config/default_spec.yaml          ← ConfigMap 复制过来       │
│  ├── config/default_user_config.yaml   ← ConfigMap 复制过来       │
│  ├── data/snapshot.json                ← daemon 写入             │
│  └── run/current/                                               │
│      ├── kubeconfig                    ← init 容器生成           │
│      ├── sichek.log                    ← daemon 日志             │
│      └── metrics.sock                  ← daemon metrics 输出     │
└─────────────────────────────────────────────────────────────────┘
```

---

## 二、三个容器详解

### 1. `sichek-init` (initContainer) — 安装器

**作用**：在主容器启动前，将 sichek 二进制和配置安装到宿主机。

**执行流程**：
1. 检查宿主机是否已有 sichek，版本是否匹配
2. 版本不匹配 → 通过 `nsenter` 进入宿主机命名空间，用 rpm/dpkg 安装 sichek + SICL 库
3. 将 ConfigMap 中的配置文件复制到宿主机 `/var/sichek/config/`
4. 生成 kubeconfig（用 ServiceAccount token），存到 `/var/sichek/run/pods/<POD_UID>/kubeconfig`
5. 创建 `current` 符号链接指向当前 pod 的运行目录
6. 生成 env 文件（sichek daemon 启动参数）
7. 清理旧 pod 目录（保留 current 和 canary）

**关键机制**：通过 `nsenter -t 1 -m -p -n -u -i --` 进入宿主机 PID 1 的所有命名空间，直接在宿主机上执行安装命令。sichek 实际运行在宿主机上（systemd 管理），不是在容器里。

### 2. `sichek` (主容器) — 守护进程管理器

**作用**：启动并保活宿主机上的 sichek daemon 服务。

**执行流程**：
1. `nsenter` 进入宿主机，执行 `sichek d start` 启动 daemon
2. `tail -F` 宿主机日志文件，转发到容器 stdout（使 `kubectl logs` 可读）
3. 每 10 秒检查 sichek 服务是否存活，如果挂了就重启

**注意**：sichek daemon 跑在宿主机上（systemd 进程），不是容器进程。这个容器只是一个"保姆"。

### 3. `sichek-exporter` (sidecar 容器) — Prometheus 指标代理

**作用**：将 sichek daemon 通过 Unix socket 输出的 metrics 转为 TCP HTTP 端口，供 Prometheus 抓取。

**执行流程**：
1. 先通过 `nsenter` 重启宿主机上的 dcgm-exporter pod（确保 GPU 指标刷新）
2. 等待 `metrics.sock` 文件就绪（sichek daemon 创建的 Unix socket）
3. 启动 `sichek exporter --metrics-socket <sock> --listen :19092`，将 socket 指标代理到 TCP 端口

**为什么需要**：sichek daemon 跑在宿主机，通过 Unix socket 输出 metrics。但 Prometheus PodMonitor 只能抓容器端口，所以需要这个容器做 socket → TCP 的桥接。

---

## 三、Volumes 定义

```yaml
volumes:
- name: sichek-host              # ① 宿主机共享目录
  hostPath:
    path: /var/sichek
    type: DirectoryOrCreate

- name: sichek-default-spec      # ② ConfigMap: 硬件规格配置
  configMap:
    name: sichek-default-spec

- name: sichek-default-user-config  # ③ ConfigMap: 用户运行配置
  configMap:
    name: sichek-default-user-config
```

### ① `sichek-host` — 宿主机共享目录

**类型**：`hostPath`，指向宿主机 `/var/sichek`，不存在则自动创建。

**作用**：容器和宿主机之间的文件通道。sichek 的所有运行时文件都在这里。

**宿主机上的目录结构**：
```
/var/sichek/
├── config/                          # 配置文件
│   ├── default_spec.yaml            # 硬件规格（从 ConfigMap 复制过来）
│   └── default_user_config.yaml     # 用户配置（从 ConfigMap 复制过来）
├── data/
│   └── snapshot.json                # 健康检查快照
├── run/
│   ├── current -> pods/<POD_UID>    # 符号链接指向当前活跃 pod
│   ├── env                          # daemon 启动环境变量
│   └── pods/
│       └── <POD_UID>/
│           ├── kubeconfig           # K8s 认证凭据
│           ├── sichek.log           # 运行日志
│           └── metrics.sock         # Prometheus 指标 Unix socket
└── scripts/                         # 辅助脚本
```

**为什么 mountPath 是 `/host/var/sichek` 而非 `/var/sichek`**：容器内 `/var/sichek/config/` 已被 ConfigMap 挂载占用（init 容器），所以宿主机的 `/var/sichek` 挂到容器的 `/host/var/sichek` 避免路径冲突。init 容器用 `nsenter` 在宿主机上执行命令时，路径是宿主机视角的 `/var/sichek/run/...`。

### ② `sichek-default-spec` — 硬件规格 ConfigMap

**类型**：ConfigMap，名称 `sichek-default-spec`。

**内容**：`config/default_spec.yaml` — nvidia GPU 型号参数、infiniband 规格、transceiver 阈值等。

**仅挂载到 init 容器**：
```yaml
volumeMounts:
- name: sichek-default-spec
  mountPath: /var/sichek/config/default_spec.yaml
  subPath: default_spec.yaml
```

**`subPath` 的作用**：只挂载 ConfigMap 中的 `default_spec.yaml` 这一个 key 到指定路径，而不是把整个 ConfigMap 挂成目录。这样容器内 `/var/sichek/config/` 目录下其他文件不受影响。

**数据流转**：
```
K8s ConfigMap (sichek-default-spec)
    ↓ subPath 挂载
容器内 /var/sichek/config/default_spec.yaml
    ↓ init 容器 cp 命令
/host/var/sichek/config/default_spec.yaml
    = 宿主机 /var/sichek/config/default_spec.yaml
    ↓ sichek daemon 读取
```

**为什么不直接让 daemon 读 ConfigMap？** sichek daemon 跑在宿主机上（systemd 进程），看不到容器内的挂载。必须通过 init 容器把文件复制到宿主机文件系统。

### ③ `sichek-default-user-config` — 用户运行配置 ConfigMap

**类型**：ConfigMap，名称 `sichek-default-user-config`。

**内容**：`config/default_user_config.yaml` — 各组件的查询间隔、缓存大小、忽略的检查项等。

**仅挂载到 init 容器**，数据流转同上。

---

## 四、各容器 volumeMounts 汇总

| 容器 | Volume | mountPath | 用途 |
|------|--------|-----------|------|
| **sichek-init** | sichek-host | `/host/var/sichek` | 写入安装包、kubeconfig、env、复制配置到宿主机 |
| **sichek-init** | sichek-default-spec | `/var/sichek/config/default_spec.yaml` (subPath) | 读 ConfigMap 中的硬件规格，再 cp 到宿主机 |
| **sichek-init** | sichek-default-user-config | `/var/sichek/config/default_user_config.yaml` (subPath) | 读 ConfigMap 中的用户配置，再 cp 到宿主机 |
| **sichek** | sichek-host | `/host/var/sichek` | 读日志文件（tail -F） |
| **sichek-exporter** | sichek-host | `/host/var/sichek` | 读 metrics.sock |

---

## 五、为什么用 ConfigMap 而不是打包到镜像

1. **配置和镜像解耦** — 更新配置不需要重新构建镜像，改 ConfigMap 后重启 pod 即可
2. **不同集群不同配置** — 不同集群的 GPU 型号、网络配置不同，ConfigMap 可以按集群定制
3. **运维可见性** — `kubectl get configmap sichek-default-spec -o yaml` 直接查看当前生效配置

---

## 六、其他 K8s 资源

### ServiceAccount + RBAC

```yaml
ServiceAccount: sa-sichek (namespace: monitoring)
ClusterRole: cluster-role-sichek
  → nodes: get, list, patch, update, watch, delete
  → pods: get, list, patch, update, watch, delete
  → pytorchjobs: get, list, patch, update, watch, delete
```

sichek 需要 patch node annotation 来上报健康状态，需要 list pods 做 GPU UUID → Pod 映射。

### PodMonitor

```yaml
PodMonitor: sichek-exporter
  → scrape interval: 15s
  → path: /metrics
  → port: metrics (19092)
```

Prometheus 通过 PodMonitor 自动发现并抓取 sichek-exporter 容器的指标端口。

---

## 七、Pod 调度策略

```yaml
hostPID: true          # 共享宿主机 PID 命名空间（nsenter 需要）
hostNetwork: true      # 共享宿主机网络（K8s API 访问 + metrics 端口）
privileged: true       # 特权模式（nsenter + 硬件访问）

affinity:
  nodeAffinity:        # 只调度到有 GPU 的节点
    requiredDuringSchedulingIgnoredDuringExecution:
      - key: scitix.ai/gpu-type
        operator: Exists

tolerations:
  - operator: Exists   # 容忍所有 taint（确保所有 GPU 节点都能部署）
```
