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

	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
)

func FillNvidiaSpec() map[string]config.NvidiaSpec {
	fmt.Println("🧩 Configuring NVIDIA spec...")
	specs := map[string]config.NvidiaSpec{}

	// Ask how many GPU types to configure
	numGpuTypes := promptInt("How many GPU types do you want to configure?", 1)

	for gpuIdx := 1; gpuIdx <= numGpuTypes; gpuIdx++ {
		fmt.Printf("\n🔧 Configuring GPU type #%d:\n", gpuIdx)

		// Basic GPU information
		gpuPciId := strings.ToLower(promptString(fmt.Sprintf("GPU PCI ID #%d (e.g. 0x232910de, 0x233510de)", gpuIdx)))
		gpuName := promptString(fmt.Sprintf("GPU name #%d (e.g. NVIDIA H20, NVIDIA H200)", gpuIdx))
		gpuNums := promptInt(fmt.Sprintf("Number of GPUs per node for %s", gpuName), 8)
		gpuMemory := promptInt(fmt.Sprintf("Memory per GPU (GB) for %s", gpuName), 96)

		// Software information
		fmt.Printf("💻 Software Configuration for %s:\n", gpuName)
		software := collector.SoftwareInfo{
			DriverVersion: promptString("Driver version", ">=550.144.03"),
			CUDAVersion:   promptString("CUDA version", ">=12.4"),
		}

		// Dependence configuration
		fmt.Printf("⚙️ Dependence Configuration for %s:\n", gpuName)
		dependence := config.Dependence{
			PcieAcs:        promptString("PCIe ACS", "disable"),
			Iommu:          promptString("IOMMU", "off"),
			NvidiaPeermem:  promptString("NV Peermem", "enable"),
			FabricManager:  promptString("NV Fabricmanager", "active"),
			CpuPerformance: promptString("CPU Performance", "enable"),
		}

		// MaxClock configuration (will be added to spec file manually)
		fmt.Printf("⏱️ MaxClock Configuration for %s:\n", gpuName)
		graphicsClock := promptInt("Graphics clock (MHz)", 1980)
		smClock := promptInt("SM clock (MHz)", 1980)
		memoryClock := promptInt("Memory clock (MHz)", 2619)
		fmt.Printf("  MaxClock: Graphics=%d, SM=%d, Memory=%d\n", graphicsClock, smClock, memoryClock)

		// NVLink configuration
		fmt.Printf("🔗 NVLink Configuration for %s:\n", gpuName)
		nvlink := collector.NVLinkStates{
			NVlinkSupported: promptBool("NVLink Supported", true),
			NvlinkNum:       int(promptInt("Active NVLink Num", 18)),
		}

		// State configuration
		fmt.Printf("🔄 State Configuration for %s:\n", gpuName)
		state := collector.StatesInfo{
			GpuPersistenceM: promptString("Persistence", "enable"),
			GpuPstate:       uint32(promptInt("Pstate", 0)),
		}

		// Memory error thresholds
		fmt.Printf("🧠 Memory Error Thresholds for %s:\n", gpuName)
		memoryErrorThreshold := config.MemoryErrorThreshold{
			RemappedUncorrectableErrors:      uint64(promptInt("Remapped Uncorrectable Errors", 512)),
			SRAMVolatileUncorrectableErrors:  uint64(promptInt("SRAM Volatile Uncorrectable Errors", 0)),
			SRAMAggregateUncorrectableErrors: uint64(promptInt("SRAM Aggregate Uncorrectable Errors", 4)),
			SRAMVolatileCorrectableErrors:    uint64(promptInt("SRAM Volatile Correctable Errors", 10000000)),
			SRAMAggregateCorrectableErrors:   uint64(promptInt("SRAM Aggregate Correctable Errors", 10000000)),
		}

		// Temperature thresholds
		fmt.Printf("🌡️ Temperature Thresholds for %s:\n", gpuName)
		temperatureThreshold := config.TemperatureThreshold{
			Gpu:    promptInt("GPU Temperature Threshold", 75),
			Memory: promptInt("Memory Temperature Threshold", 95),
		}

		// Performance metrics
		fmt.Printf("⚡ Performance Metrics for %s:\n", gpuName)
		perf := config.PerfMetrics{
			NcclAllReduceBw: promptFloat("NCCL all-reduce bandwidth (GB/s)", 470),
		}

		// Create the NVIDIA spec for this GPU type
		specs[gpuPciId] = config.NvidiaSpec{
			Name:                 gpuName,
			GpuNums:              gpuNums,
			GpuMemory:            gpuMemory,
			Software:             software,
			Dependence:           dependence,
			Nvlink:               nvlink,
			State:                state,
			MemoryErrorThreshold: memoryErrorThreshold,
			TemperatureThreshold: temperatureThreshold,
			Perf:                 perf,
		}

		fmt.Printf("✅ GPU type #%d (%s - %s) configured successfully\n", gpuIdx, gpuPciId, gpuName)
	}

	fmt.Printf("\n🎉 Configured %d GPU type(s) successfully\n", numGpuTypes)
	return specs
}
