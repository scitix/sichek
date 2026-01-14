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
package collector

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type IBSoftWareInfo struct {
	OFEDVer      string   `json:"ofed_ver" yaml:"ofed_ver"`
	KernelModule []string `json:"kernel_module" yaml:"kernel_module"`
}

// Collect collects all software information and fills the struct
func (sw *IBSoftWareInfo) Collect(ctx context.Context) {
	sw.OFEDVer = strings.TrimPrefix(sw.GetOFEDInfo(ctx), "rdma-core:")
	sw.KernelModule = sw.GetKernelModule()
}

// GetOFEDInfo gets OFED information
func (sw *IBSoftWareInfo) GetOFEDInfo(ctx context.Context) string {

	if _, err := exec.LookPath("ofed_info"); err == nil {
		if output, err := exec.CommandContext(ctx, "ofed_info", "-s").Output(); err == nil {
			if ver := strings.Split(string(output), ":")[0]; ver != "" {
				return ver
			}
		}
	}

	if data, err := os.ReadFile("/sys/module/mlx5_core/version"); err == nil {
		if ver := strings.TrimSpace(string(data)); ver != "" {
			return fmt.Sprintf("rdma_core:%s", ver)
		}
	}
	return "UnKnown"
}

// GetKernelModule gets kernel modules
func (sw *IBSoftWareInfo) GetKernelModule() []string {
	preInstallModule := []string{
		"rdma_ucm",
		"rdma_cm",
		"ib_ipoib",
		"mlx5_core",
		"mlx5_ib",
		"ib_uverbs",
		"ib_umad",
		"ib_cm",
		"ib_core",
		"mlxfw"}
	if utils.IsNvidiaGPUExist() {
		preInstallModule = append(preInstallModule, "nvidia_peermem")
	}

	var installedModule []string
	for _, module := range preInstallModule {
		installed := IsModuleLoaded(module)
		if installed {
			installedModule = append(installedModule, module)
		} else {
			logrus.WithField("component", "infiniband").Errorf("Fail to load the kernel module %s", module)
		}
	}

	return installedModule
}
