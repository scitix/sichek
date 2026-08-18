/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// gpu_probe: 单卡 GPU 功能自检探针。
// 退出码约定（sichek collector 依赖，勿改语义）:
//   0 PASS     kernel 结果逐元素校验通过
//   1 FAIL     结果 mismatch —— 卡算错了
//   2 SKIP     主动让路：free% < min-free-pct 或 设备忙/显存不足
//   3 ENV_ERR  其余 CUDA API 失败 —— CUDA 环境异常
#include <iostream>
#include <cstdlib>
#include <cstring>
#include <vector>
#include <cuda_runtime.h>

enum ProbeExit { PROBE_PASS = 0, PROBE_FAIL = 1, PROBE_SKIP = 2, PROBE_ENV_ERR = 3 };

// CUDA API 失败一律归为环境错(3)，与"算错(1)"区分。
#define CHECK_CUDA(call) { \
    cudaError_t err = call; \
    if (err != cudaSuccess) { \
        std::cerr << "CUDA_ERR@" << __FILE__ << ":" << __LINE__ \
                  << " code=" << err << "(" << cudaGetErrorString(err) \
                  << ") op: " #call << std::endl; \
        std::cout << "RESULT=ENV_ERR device=" << deviceId \
                  << " detail=" << cudaGetErrorString(err) << std::endl; \
        exit(PROBE_ENV_ERR); \
    }}

__global__ void basic_validation_kernel(int* a, int* b, int* c, int n) {
    int tid = threadIdx.x + blockIdx.x * blockDim.x;
    if (tid < n) c[tid] = a[tid] * 2 + b[tid];
}

int runProbe(int deviceId, float minFreePct) {
    cudaSetDevice(deviceId);

    size_t freeMem = 0, totalMem = 0;
    cudaError_t memErr = cudaMemGetInfo(&freeMem, &totalMem);
    if (memErr == cudaErrorMemoryAllocation || memErr == cudaErrorDevicesUnavailable) {
        std::cout << "RESULT=SKIP device=" << deviceId
                  << " detail=" << cudaGetErrorString(memErr) << std::endl;
        return PROBE_SKIP;
    } else if (memErr != cudaSuccess) {
        std::cout << "RESULT=ENV_ERR device=" << deviceId
                  << " detail=" << cudaGetErrorString(memErr) << std::endl;
        return PROBE_ENV_ERR;
    }
    float freePct = totalMem ? (float)freeMem / totalMem * 100.0f : 0.0f;
    if (freePct < minFreePct) {
        std::cout << "RESULT=SKIP device=" << deviceId
                  << " free_pct=" << freePct << " min=" << minFreePct << std::endl;
        return PROBE_SKIP;
    }

    const int test_size = 1 << 20;      // 1M 元素
    const int block_size = 256;
    std::vector<int> h_a(test_size), h_b(test_size), h_c(test_size, 0);
    int *d_a, *d_b, *d_c;
    for (int i = 0; i < test_size; ++i) { h_a[i] = i; h_b[i] = i % 100; }

    CHECK_CUDA(cudaMalloc(&d_a, test_size * sizeof(int)));
    CHECK_CUDA(cudaMalloc(&d_b, test_size * sizeof(int)));
    CHECK_CUDA(cudaMalloc(&d_c, test_size * sizeof(int)));
    CHECK_CUDA(cudaMemcpy(d_a, h_a.data(), test_size * sizeof(int), cudaMemcpyHostToDevice));
    CHECK_CUDA(cudaMemcpy(d_b, h_b.data(), test_size * sizeof(int), cudaMemcpyHostToDevice));

    dim3 grid((test_size + block_size - 1) / block_size);
    basic_validation_kernel<<<grid, block_size>>>(d_a, d_b, d_c, test_size);
    CHECK_CUDA(cudaGetLastError());
    CHECK_CUDA(cudaDeviceSynchronize());
    CHECK_CUDA(cudaMemcpy(h_c.data(), d_c, test_size * sizeof(int), cudaMemcpyDeviceToHost));

    bool valid = true;
    for (int i = 0; i < test_size; ++i) {
        int expected = h_a[i] * 2 + h_b[i];
        if (h_c[i] != expected) {
            std::cerr << "MISMATCH@i=" << i << " expect=" << expected << " got=" << h_c[i] << std::endl;
            valid = false;
            break;
        }
    }
    cudaFree(d_a); cudaFree(d_b); cudaFree(d_c);

    if (valid) { std::cout << "RESULT=PASS device=" << deviceId << std::endl; return PROBE_PASS; }
    std::cout << "RESULT=FAIL device=" << deviceId << std::endl;
    return PROBE_FAIL;
}

int main(int argc, char* argv[]) {
    int deviceId = -1;
    float minFreePct = 50.0f;
    for (int i = 1; i < argc; ++i) {
        if (!strcmp(argv[i], "-d") && i + 1 < argc) deviceId = atoi(argv[++i]);
        else if (!strcmp(argv[i], "--min-free-pct") && i + 1 < argc) minFreePct = atof(argv[++i]);
    }
    if (deviceId < 0) {
        std::cerr << "usage: gpu_probe -d <device> [--min-free-pct <pct>]" << std::endl;
        return PROBE_ENV_ERR;
    }
    int deviceCount = 0;
    if (cudaGetDeviceCount(&deviceCount) != cudaSuccess || deviceId >= deviceCount) {
        std::cout << "RESULT=ENV_ERR device=" << deviceId << " detail=invalid_device_or_init_failed" << std::endl;
        return PROBE_ENV_ERR;
    }
    return runProbe(deviceId, minFreePct);
}
