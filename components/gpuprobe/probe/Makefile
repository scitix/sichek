# 单独编译探针，不接入主 `make`（主流程保持纯 Go、离线可建）。
# 需在带 CUDA toolchain 的主机上手动运行，产物提交到 ../bin/。
NVCC ?= nvcc
# 覆盖主流架构：Ampere(80) / Hopper(90) / Blackwell(100,120)。按需增减。
GENCODE ?= -gencode arch=compute_80,code=sm_80 \
           -gencode arch=compute_90,code=sm_90 \
           -gencode arch=compute_100,code=sm_100 \
           -gencode arch=compute_120,code=sm_120 \
           -gencode arch=compute_90,code=compute_90
# 静态链 cudart：运行期只依赖驱动 libcuda.so.1，不绑 CUDA runtime 版本。
CUDA_LINK ?= -lcudart_static -ldl -lrt -lpthread

.PHONY: amd64 arm64
amd64:
	$(NVCC) $(GENCODE) -O2 gpu_probe.cu -o ../bin/gpu_probe.amd64 $(CUDA_LINK)
arm64:
	$(NVCC) $(GENCODE) -O2 gpu_probe.cu -o ../bin/gpu_probe.arm64 $(CUDA_LINK)
