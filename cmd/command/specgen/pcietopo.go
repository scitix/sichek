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
	"strings"

	"github.com/scitix/sichek/components/pcie/config"
)

func FillPcieTopoSpec() map[string]config.PcieTopoSpec {
	fmt.Println("🧮 Configuring PCIe topology...")
	specs := map[string]config.PcieTopoSpec{}

	// Ask how many GPU Node types to configure
	numGpuIds := promptInt("How many GPU Node types do you want to configure?", 1)

	for gpuIdx := 1; gpuIdx <= numGpuIds; gpuIdx++ {
		fmt.Printf("\n🔧 Configuring GPU Node type #%d:\n", gpuIdx)

		// Get GPU PCI ID
		gpuPciId := strings.ToLower(promptString(fmt.Sprintf("GPU PCI ID #%d (e.g. 0x233510de)", gpuIdx)))

		// Configure NUMA nodes for this GPU
		fmt.Printf("📋 NUMA Configuration for %s:\n", gpuPciId)
		numNUMA := promptInt("Number of NUMA nodes per Node", 2)
		var numaList []*config.NumaConfig

		for i := 0; i < numNUMA; i++ {
			fmt.Printf("  NUMA Node %d:\n", i)
			numaList = append(numaList, &config.NumaConfig{
				NodeID:   uint64(i),
				GPUCount: promptInt(fmt.Sprintf("    GPU count for NUMA %d", i), 4),
				IBCount:  promptInt(fmt.Sprintf("    IB count for NUMA %d", i), 2),
			})
		}

		// Configure PCIe switches for this GPU
		fmt.Printf("🔌 PCIe Switch Configuration for %s:\n", gpuPciId)
		numSwitch := promptInt("Number of Common PCIe switch types", 1)
		var switchList []*config.PciSwitch

		for i := 0; i < numSwitch; i++ {
			fmt.Printf("  PCIe Switch Type %d:\n", i+1)
			switchList = append(switchList, &config.PciSwitch{
				GPU:   promptInt("    GPU per switch", 2),
				IB:    promptInt("    IB per switch", 1),
				Count: promptInt("    Number of switches", 4),
			})
		}

		// Create the topology spec for this GPU PCI ID
		specs[gpuPciId] = config.PcieTopoSpec{
			NumaConfig:        numaList,
			PciSwitchesConfig: switchList,
		}

		fmt.Printf("✅ GPU PCI ID #%d (%s) spec configured successfully\n", gpuIdx, gpuPciId)
	}

	fmt.Printf("\n🎉 Configured %d GPU PCI ID topology(s) successfully\n", numGpuIds)
	return specs
}
