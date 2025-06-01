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
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	hcaConfig "github.com/scitix/sichek/components/hca/config"
)

func TestGetHCASpec(t *testing.T) {
	// Create temporary spec file
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to close temp spec file: %v", err)
		}
	}(specFile.Name())
	// Write sample data to the temporary files
	specData := `
nvidia:
  "0x233010de":
    name: NVIDIA H100 80GB HBM3
    gpu_nums: 8
    gpu_memory: 80
    pcie:
      pci_gen: 5
      pci_width: 16
    software:
      driver_version: "535.129.03"
      cuda_version: "12.0"
      vbios_version: "96.00.89.00.01"
      nvidiafabric_manager: "535.129.03"
    dependence:
      pcie-acs: disable
      iommu: disable
      nv-peermem: enable
      nv_fabricmanager: active
      cpu_performance: enable
    MaxClock:
      Graphics: 1410 # MHz
      Memory: 1593 # MHz
      SM: 1410 # MHz
    nvlink:
      nvlink_supported: true
      active_nvlink_num: 12
      total_replay_errors: 0
      total_recovery_errors: 0
      total_crc_errors: 0
    state:
      persistence: enable
      pstate: 0
    memory_errors_threshold:
      remapped_uncorrectable_errors: 512
      sram_volatile_uncorrectable_errors: 0
      sram_aggregate_uncorrectable_errors: 4
      sram_volatile_correctable_errors: 10000000
      sram_aggregate_correctable_errors: 10000000
    temperature_threshold:
      gpu: 75
      memory: 95
infiniband:
  cluster_name:
    ib_devs:
      - mlx5_0
      - mlx5_1
    eth_devs:
      - ibs18
      - ibs20
    sw_deps:
      kernel_module:
        - "rdma_ucm"
        - "rdma_cm"
        - "ib_ipoib"
        - "mlx5_core"
        - "mlx5_ib"
        - "ib_uverbs"
        - "ib_umad"
        - "ib_cm"
        - "ib_core"
        - "mlxfw"
      ofed_ver: "MLNX_OFED_LINUX-23.10-1.1.9.0"
    hca_specs:
      MT_0000001119:
        hardware:
          hca_type: "MT4129"
          board_id: "MT_0000001119"
          fw_ver: "28.42.1000"
          vpd: "HPE InfiniBand NDR200/Ethernet 200Gb 1-port OSFP PCIe5 x16 MCX75310AAS-HEAT Adapter"
          net_port: 1
          port_speed: "200 Gb/sec (2X NDR)"
          phy_state: "LinkUp"
          port_state: "ACTIVE"
          net_operstate: "down"
          link_layer: "InfiniBand"
          pcie_width: "16"
          pcie_speed: "16.0 GT/s PCIe"
          pcie_tree_width: "32"
          pcie_tree_speed: "16"
          pcie_acs: "disable"
          pcie_mrr: "4096"
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}
	ibSpecs := &InfinibandSpecs{}
	err = ibSpecs.tryLoadFromFile(specFile.Name())
	if err != nil {
		t.Fatalf("Failed to get spec: %v", err)
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(ibSpecs, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	if len(ibSpecs.Specs) != 1 {
		t.Fatalf("Expected spec at least have 1 entry, got %d", len(ibSpecs.Specs))
	}
	if _, ok := ibSpecs.Specs["cluster_name"]; !ok {
		t.Fatalf("Expected spec to have key 'cluster_name', it doesn't exist")
	}
	if _, ok := ibSpecs.Specs["cluster_name"].HCAs["MT_0000001119"]; !ok {
		t.Fatalf("Expected spec to have key 'MT_0000001119', it doesn't exist")
	}
	hcaSpec := ibSpecs.Specs["cluster_name"].HCAs["MT_0000001119"]
	if hcaSpec.Hardware.BoardID != "MT_0000001119" {
		t.Fatalf("Expected BoardID 'MT_0000001119', got '%s'", hcaSpec.Hardware.BoardID)
	}
	if hcaSpec.Hardware.FWVer != "28.42.1000" {
		t.Fatalf("Expected FwVer '28.42.1000', got '%s'", hcaSpec.Hardware.FWVer)
	}
	if hcaSpec.Hardware.PCIEWidth != "16" {
		t.Fatalf("Expected PcieWidth '16', got '%s'", hcaSpec.Hardware.PCIEWidth)
	}
	if hcaSpec.Hardware.PCIESpeed != "16.0 GT/s PCIe" {
		t.Fatalf("Expected PcieSpeed '16.0 GT/s PCIe', got '%s'", hcaSpec.Hardware.PCIESpeed)
	}
}

func TestGetDefaultHcaSpec(t *testing.T) {
	// Create temporary files for testing
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to close temp spec file: %v", err)
		}
	}(specFile.Name())

	// Write sample data to the temporary files
	specData := `
nvidia:
  "0x233010de":
    name: NVIDIA H100 80GB HBM3
    gpu_nums: 8
    gpu_memory: 80
    pcie:
      pci_gen: 5
      pci_width: 16
    software:
      driver_version: "535.129.03"
      cuda_version: "12.0"
      vbios_version: "96.00.89.00.01"
      nvidiafabric_manager: "535.129.03"
    dependence:
      pcie-acs: disable
      iommu: disable
      nv-peermem: enable
      nv_fabricmanager: active
      cpu_performance: enable
    MaxClock:
      Graphics: 1410 # MHz
      Memory: 1593 # MHz
      SM: 1410 # MHz
    nvlink:
      nvlink_supported: true
      active_nvlink_num: 12
      total_replay_errors: 0
      total_recovery_errors: 0
      total_crc_errors: 0
    state:
      persistence: enable
      pstate: 0
    memory_errors_threshold:
      remapped_uncorrectable_errors: 512
      sram_volatile_uncorrectable_errors: 0
      sram_aggregate_uncorrectable_errors: 4
      sram_volatile_correctable_errors: 10000000
      sram_aggregate_correctable_errors: 10000000
    temperature_threshold:
      gpu: 75
      memory: 95
infiniband:
  default:
    ib_devs:
      - mlx5_0
      - mlx5_1
    eth_devs:
      - ibs18
      - ibs20
    sw_deps:
      kernel_module:
        - "rdma_ucm"
        - "rdma_cm"
        - "ib_ipoib"
        - "mlx5_core"
        - "mlx5_ib"
        - "ib_uverbs"
        - "ib_umad"
        - "ib_cm"
        - "ib_core"
        - "mlxfw"
      ofed_ver: "MLNX_OFED_LINUX-23.10-1.1.9.0"
    hca_specs: 
      MT_0000000970: {}
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	// Test the LoadSpec function
	nic, err := hcaConfig.GetBoardIDs()
	if err != nil {
		t.Skip("Skipping test due to error in GetBoardIDs: ", err)
	}
	nicSet := make(map[string]struct{})
	for _, n := range nic {
		nicSet[n] = struct{}{}
	}
	if _, ok := nicSet["MT_0000000970"]; !ok {
		t.Skip("Skipping test because MT_0000000970 is not in the list of NICs")
	}
	spec, err := LoadSpec(specFile.Name())
	if err != nil {
		t.Fatalf("Failed to get spec: %v", err)
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	t.Logf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	hcaSpecs := spec.HCAs
	if _, ok := hcaSpecs["MT_0000000970"]; !ok {
		t.Fatalf("Expected spec to have key 'MT_0000000970', it doesn't exist")
	}
	hcaSpec := hcaSpecs["MT_0000000970"]
	if hcaSpec.Hardware.BoardID != "MT_0000000970" {
		t.Fatalf("Expected BoardID 'MT_0000000970', got '%s'", hcaSpec.Hardware.BoardID)
	}
	if hcaSpec.Hardware.FWVer != "28.39.2048" {
		t.Fatalf("Expected FwVer '28.39.2048', got '%s'", hcaSpec.Hardware.FWVer)
	}
	if hcaSpec.Hardware.PCIEWidth != "16" {
		t.Fatalf("Expected PcieWidth '16', got '%s'", hcaSpec.Hardware.PCIEWidth)
	}
	if hcaSpec.Hardware.PCIESpeed != "32.0 GT/s PCIe" {
		t.Fatalf("Expected PcieSpeed '32.0 GT/s PCIe', got '%s'", hcaSpec.Hardware.PCIESpeed)
	}
}

func TestGetClusterInfinibandSpec(t *testing.T) {
	clusterSpec, err := LoadSpec("")
	if err != nil {
		t.Fatalf("Failed to get spec: %v", err)
	}
	jsonData, err := json.MarshalIndent(clusterSpec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))
}
