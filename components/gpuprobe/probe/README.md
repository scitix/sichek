# gpu_probe

单卡 GPU 计算功能自检探针，由 sichek `gpuprobe` 组件的 collector 以子进程方式 exec 调用（`-d <device_id> [--min-free-pct <pct>]`），据其退出码与 stdout 判定该卡是否算力正常。

## 退出码

| Exit code | 含义 | 触发条件 |
|---|---|---|
| `0` | PASS | kernel 计算结果逐元素校验通过 |
| `1` | FAIL | 结果 mismatch —— 卡算错了（真故障信号） |
| `2` | SKIP | 主动让路：显存 free% 低于 `--min-free-pct`，或设备忙 / 显存不足（`cudaErrorMemoryAllocation` / `cudaErrorDevicesUnavailable`） |
| `3` | ENV_ERR | 其余 CUDA API 调用失败（驱动 / 初始化 / 无效设备号等），CUDA 环境异常而非算力故障 |

collector 依赖这套退出码语义做判定，修改探针逻辑时不要改变其含义。

## 构建

主 Go 工程的 `make` **不会**构建本探针（保持主流程纯 Go、离线可建）。需在装有目标架构 CUDA toolchain（`nvcc`）的主机上手动执行：

```bash
cd components/gpuprobe/probe
make amd64   # 需要 x86_64 主机 + nvcc
make arm64   # 需要 aarch64 主机（或交叉工具链）+ nvcc
```

产物写入 `../bin/gpu_probe.amd64` / `../bin/gpu_probe.arm64`。

## 提交编译产物

`bin/` 目录被 `.gitignore` 忽略，需强制入仓：

```bash
git add -f ../bin/gpu_probe.amd64 ../bin/gpu_probe.arm64
git commit -m "feat(gpuprobe): 预编译探针 ELF 入仓(amd64/arm64)"
```

## 静态链接说明

`Makefile` 用 `-lcudart_static` 静态链接 CUDA runtime，运行期只依赖驱动提供的 `libcuda.so.1`，与目标节点安装的 CUDA runtime 版本无关，避免版本不匹配导致探针启动失败。

## 何时需要重新编译

- 修改了 `gpu_probe.cu` 的逻辑
- 集群引入了新的 GPU 架构（需要在 `Makefile` 的 `GENCODE` 中补充对应 `-gencode arch=compute_XX,code=sm_XX`）
- 目标节点驱动发生重大变更，需要重新验证兼容性

修改 `GENCODE` 时保持覆盖所有在线集群的 GPU 架构；当前默认覆盖 Ampere(80) / Hopper(90) / Blackwell(100,120)。

## 打包（随 sichek 发布）

amd64 ELF 已入仓，打包接线**已完成**：

### 1. `.goreleaser.yaml`（已接）

nfpm 的 `contents:` 里已追加：

```yaml
    - src: ./components/gpuprobe/bin/gpu_probe.amd64
      dst: /var/sichek/bin/gpu_probe
      file_info:
        mode: 0755
```

goreleaser 当前只构建 linux/amd64。

### 2. `docker/Dockerfile`（无需改）

镜像通过 `dpkg -i sichek_*.deb` 安装，deb 已由上面的 nfpm 条目带上 `/var/sichek/bin/gpu_probe`，**Docker 镜像自动继承，无需单独 `COPY`**。

### arm64（仅 Grace 系需要）

若有 Grace+Blackwell（GB200/GB300 等 aarch64 宿主）机型，需另编 `gpu_probe.arm64`（arm64 CUDA devel 镜像 + qemu，或 arm64 宿主），并为其加一条平行的 nfpm `contents`（`dst` 同为 `/var/sichek/bin/gpu_probe`，随 arm64 构建产物走）。纯 x86 宿主（HGX/DGX B300、H200 服务器、5090 工作站）用 amd64 即可。

### 重新编译（用 docker，不需 GPU）

**务必用 ubuntu20.04(glibc 2.31)基础镜像编译**,产物只需 GLIBC_2.17,可在全 fleet
运行(含 Anolis/al8 等老 glibc 节点,glibc 2.32);用 ubuntu22.04 编会引入 GLIBC_2.34
依赖(pthread 并入 libc),在老 glibc 节点报 `version 'GLIBC_2.34' not found`。
**不要用 CUDA 13**(静态 cudart 要求驱动 ≥580,会砸掉老驱动节点;详见组件文档)。

```bash
mkdir -p components/gpuprobe/bin
docker run --rm -v "$(pwd)/components/gpuprobe:/work" -w /work/probe \
  nvidia/cuda:12.8.1-devel-ubuntu20.04 \
  nvcc -gencode arch=compute_80,code=sm_80 \
       -gencode arch=compute_90,code=sm_90 \
       -gencode arch=compute_100,code=sm_100 \
       -gencode arch=compute_120,code=sm_120 \
       -gencode arch=compute_90,code=compute_90 \
       -O2 gpu_probe.cu -o ../bin/gpu_probe.amd64 \
       -lcudart_static -ldl -lrt -lpthread
git add components/gpuprobe/bin/gpu_probe.amd64
# B300 是 sm_103,由 compute_90 PTX 前向 JIT 覆盖(无需 sm_103 SASS)。
```
