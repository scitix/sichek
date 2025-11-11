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
package specgen

import (
	"fmt"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
)

func FillInfinibandSpec() map[string]config.InfinibandSpec {
	fmt.Println("🔗 Configuring InfiniBand spec...")

	specs := map[string]config.InfinibandSpec{}

	devs := map[string]string{}
	num := promptInt("Number of IB devices per node", 4)
	for i := 0; i < num; i++ {
		dev := promptString(fmt.Sprintf("  Device mlx5_%d name:", i), fmt.Sprintf("mlx5_%d", i))
		ifname := promptString(fmt.Sprintf("  Interface for %s:", dev), fmt.Sprintf("eth%d", i))
		devs[dev] = ifname
	}

	ofed := promptString("OFED version (e.g. OFED-internal-23.10-1.1.9)", "OFED-internal-23.10-1.1.9")

	specs["default"] = config.InfinibandSpec{
		IBPFDevs: devs,
		IBSoftWareInfo: &collector.IBSoftWareInfo{
			KernelModule: []string{
				"rdma_ucm",
				"rdma_cm",
				"ib_ipoib",
				"mlx5_core",
				"mlx5_ib",
				"ib_uverbs",
				"ib_umad",
				"ib_cm",
				"ib_core",
				"mlxfw",
			},
			OFEDVer: ofed,
		},
		PCIeACS: "disable",
	}
	return specs
}
