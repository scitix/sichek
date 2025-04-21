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

	"github.com/scitix/sichek/components/hca/config"
)

func TestGetHCASpec(t *testing.T) {
	// Create temporary files for testing
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Errorf("Failed to remove temp spec file: %v", err)
		}
	}(specFile.Name())

	// Write sample data to the temporary files
	specData := `
hca:
  hardwares:
    MT_0000000971:
      hca_type: "MT4129"
      board_id: "MT_0000000970"
      fw_ver: "28.39.2048"
      vpd: "P45645-001 / HPE InfiniBand NDR 1-port OSFP PCIe5 x16 MCX75310AAS-NEAT Adapter"
      net_port: 1
      port_speed: "400 Gb/sec (4X NDR)"
      phy_state: "LinkUp"
      port_state: "ACTIVE"
      net_operstate: "down"
      link_layer: "InfiniBand"
      pcie_width: "16"
      pcie_speed: "32"
      pcie_tree_width: "32"
      pcie_tree_speed: "16"
      pcie_acs: "disable"
      pcie_mrr: "4096"
    MT_0000001119:
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

	// Test the GetHCASpec function
	hcaSpecs := &config.HCASpecConfig{}
	t.Logf("Reading from %s", specFile.Name())
	err = hcaSpecs.LoadSpecConfigFromYaml(specFile.Name())
	if err != nil {
		t.Fatalf("GetHCASpec() returned an error: %v", err)
	}

	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(hcaSpecs, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	if len(hcaSpecs.HcaSpec.HCAHardwares) < 3 {
		t.Fatalf("Expected spec at least have 3 entry, got %d", len(hcaSpecs.HcaSpec.HCAHardwares))
	}
	if _, ok := hcaSpecs.HcaSpec.HCAHardwares["MT_0000000971"]; !ok {
		t.Fatalf("Expected spec to have key 'MT_0000000971', it doesn't exist")
	}
	if _, ok := hcaSpecs.HcaSpec.HCAHardwares["MT_0000001119"]; !ok {
		t.Fatalf("Expected spec to have key 'MT_0000001119', it doesn't exist")
	}
	if hcaSpecs.HcaSpec.HCAHardwares["MT_0000000971"].HCAType != "MT4129" {
		t.Fatalf("Expected Spec.HCAType to be 'MT4129', got '%s'", hcaSpecs.HcaSpec.HCAHardwares["MT_0000000971"].HCAType)
	}
}

func TestGetInfinibandSpec(t *testing.T) {
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
  0x233010de:
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
  clusters:
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
	ibSpec := &InfinibandSpecConfig{}
	err = ibSpec.LoadSpecConfigFromYaml(specFile.Name())
	if err != nil {
		t.Fatalf("Failed to get spec: %v", err)
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(ibSpec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	if len(ibSpec.InfinibandSpec.Clusters) < 2 {
		t.Fatalf("Expected spec at least have 2 entry, got %d", len(ibSpec.InfinibandSpec.Clusters))
	}
	if _, ok := ibSpec.InfinibandSpec.Clusters["cluster_name"]; !ok {
		t.Fatalf("Expected spec to have key 'cluster_name', it doesn't exist")
	}
}

func TestGetDefaultInfinibandSpec(t *testing.T) {
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
  0x233010de:
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
  clusters:
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

	// Test the GetHCASpec function
	ibSpec := &InfinibandSpecConfig{}
	err = ibSpec.LoadSpecConfigFromYaml(specFile.Name())
	if err != nil {
		t.Fatalf("Failed to get spec: %v", err)
	}
	// Convert the config struct to a pretty-printed JSON string and print it
	jsonData, err := json.MarshalIndent(ibSpec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}

	fmt.Printf("spec JSON:\n%s\n", string(jsonData))

	// Validate the returned spec
	if len(ibSpec.InfinibandSpec.Clusters) < 1 {
		t.Fatalf("Expected spec at least have 1 entry, got %d", len(ibSpec.InfinibandSpec.Clusters))
	}
	if _, ok := ibSpec.InfinibandSpec.Clusters["default"]; !ok {
		t.Fatalf("Expected spec to have key 'default', it doesn't exist")
	}
}

func TestGetClusterInfinibandSpec(t *testing.T) {
	ibSpec := &InfinibandSpecConfig{}
	err := ibSpec.LoadSpecConfigFromYaml("")
	if err != nil {
		t.Fatalf("Failed to get spec: %v", err)
	}
	clusterSpec, err := ibSpec.GetClusterInfinibandSpec()
	if err != nil {
		t.Fatalf("Failed to get cluster spec: %v", err)
	}
	jsonData, err := json.MarshalIndent(clusterSpec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	fmt.Printf("spec JSON:\n%s\n", string(jsonData))
}
