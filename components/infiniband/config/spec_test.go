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
	"os"
	"testing"

	hcaConfig "github.com/scitix/sichek/components/hca/config"
	"github.com/scitix/sichek/pkg/utils"
)

// TestInfinibandSpecNoHcaFromYaml verifies that infiniband section no longer contains hca_specs;
// HCAs are not unmarshaled from YAML (filled by FilterSpec from top-level "hca" only).
func TestInfinibandSpecNoHcaFromYaml(t *testing.T) {
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer os.Remove(specFile.Name())
	specData := `
nvidia: {}
infiniband:
  cluster_name:
    ib_devs:
      mlx5_0: ibs18
      mlx5_1: ibs20
    sw_deps:
      kernel_module: ["rdma_ucm", "mlx5_core"]
      ofed_ver: "MLNX_OFED_LINUX-23.10-1.1.9.0"
    pcie_acs: "disable"
hca:
  MT_0000001119:
    hardware:
      hca_type: "MT4129"
      board_id: "MT_0000001119"
      fw_ver: "28.42.1000"
      pcie_width: "16"
      pcie_speed: "16.0 GT/s PCIe"
      pcie_tree_width: "32"
      pcie_tree_speed: "16"
      pcie_acs: "disable"
      pcie_mrr: "4096"
    perf: {}
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}
	ibSpecs := &InfinibandSpecs{}
	if err := utils.LoadFromYaml(specFile.Name(), ibSpecs); err != nil {
		t.Fatalf("LoadFromYaml: %v", err)
	}
	if len(ibSpecs.Specs) != 1 {
		t.Fatalf("Expected spec to have 1 entry, got %d", len(ibSpecs.Specs))
	}
	if _, ok := ibSpecs.Specs["cluster_name"]; !ok {
		t.Fatalf("Expected spec key 'cluster_name'")
	}
	// HCAs are not read from YAML (infiniband has no hca_specs); must be nil after unmarshal
	if ibSpecs.Specs["cluster_name"].HCAs != nil {
		t.Fatalf("Expected HCAs to be nil when loaded from YAML (infiniband does not contain hca_specs)")
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
infiniband:
  default:
    ib_devs:
      mlx5_0: ibs18
      mlx5_1: ibs20
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
    pcie_acs: "disable"
hca:
  MT_0000000970: {}
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	// Test the LoadSpec function
	_, nic, err := hcaConfig.GetIBPFBoardIDs()
	if err != nil {
		t.Skip("Skipping test due to error in GetIBPFBoardIDs: ", err)
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
	// LoadSpec requires a non-empty file path; use a temp spec file with infiniband + top-level hca
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer os.Remove(specFile.Name())
	specData := `
nvidia: {}
infiniband:
  default:
    ib_devs:
      mlx5_0: ib0
    sw_deps:
      kernel_module: ["mlx5_core"]
      ofed_ver: ">=24.10"
    pcie_acs: "disable"
hca:
  MT_0000000838: {}
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write temp spec file: %v", err)
	}
	specFile.Close()

	clusterSpec, err := LoadSpec(specFile.Name())
	if err != nil {
		t.Fatalf("Failed to get spec: %v", err)
	}
	jsonData, err := json.MarshalIndent(clusterSpec, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config to JSON: %v", err)
	}
	t.Logf("spec JSON:\n%s", string(jsonData))
}

// TestLoadSpecFillsHCAsFromTopLevelHca verifies that LoadSpec (FilterSpec) fills InfinibandSpec.HCAs
// from the top-level "hca" section for each board ID on the host.
func TestLoadSpecFillsHCAsFromTopLevelHca(t *testing.T) {
	specData := `
nvidia: {}
infiniband:
  taihua: &ib_taihua
    ib_devs:
      mlx5_0: ib0
      mlx5_1: ib1
    sw_deps:
      kernel_module: ["rdma_ucm", "mlx5_core"]
      ofed_ver: ">=MLNX_OFED_LINUX-24.10-2.1.8.0"
    pcie_acs: "disable"
  default:
    <<: *ib_taihua
hca:
  MT_0000000838:
    hardware:
      hca_type: "MT4129"
      board_id: "MT_0000000838"
      fw_ver: ">=28.39.2048"
      vpd: "NVIDIA ConnectX-7 HHHL Adapter card"
      net_port: 1
      port_speed: "400 Gb/sec (4X NDR)"
      phy_state: "LinkUp"
      port_state: "ACTIVE"
      net_operstate: "down"
      link_layer: "InfiniBand"
      pcie_width: "16"
      pcie_speed: "32.0 GT/s PCIe"
      pcie_tree_width: "16"
      pcie_tree_speed: "32"
      pcie_acs: "disable"
      pcie_mrr: "4096"
    perf:
      one_way_bw: 360
      avg_latency_us: 10
  MT_0000000834:
    hardware:
      hca_type: "0"
      board_id: "MT_0000000834"
      fw_ver: ">=28.39.1002"
      net_port: 1
      port_speed: "200 Gb/sec (2X NDR)"
      pcie_width: "16"
      pcie_speed: "32.0 GT/s PCIe"
      pcie_tree_width: "16"
      pcie_tree_speed: "32"
      pcie_acs: "disable"
      pcie_mrr: "4096"
    perf: {}
  MT_2420110034:
    hardware:
      board_id: "MT_2420110034"
      fw_ver: ">=28.39.2048"
      port_speed: "200 Gb/sec (2X NDR)"
      pcie_width: "16"
      pcie_mrr: "4096"
    perf: {}
`
	specFile, err := os.CreateTemp("", "spec_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp spec file: %v", err)
	}
	defer os.Remove(specFile.Name())
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write temp spec file: %v", err)
	}
	if err := specFile.Close(); err != nil {
		t.Fatalf("Failed to close temp spec file: %v", err)
	}

	spec, err := LoadSpec(specFile.Name())
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	// HCAs are filled from top-level hca for board IDs on the host; may be empty if no IB devices
	if spec.HCAs == nil {
		t.Fatal("spec.HCAs should be initialized (possibly empty)")
	}
	// If host has IB devices, spot-check one that we defined in top-level hca
	for boardID, hca := range spec.HCAs {
		if hca == nil {
			continue
		}
		if hca.Hardware.BoardID != boardID {
			t.Errorf("HCAs[%s].Hardware.BoardID: want %s, got %s", boardID, boardID, hca.Hardware.BoardID)
		}
		if boardID == "MT_0000000838" {
			if hca.Hardware.FWVer != ">=28.39.2048" {
				t.Errorf("HCAs[MT_0000000838].Hardware.FWVer: want >=28.39.2048, got %s", hca.Hardware.FWVer)
			}
			if hca.Hardware.HCAType != "MT4129" {
				t.Errorf("HCAs[MT_0000000838].Hardware.HCAType: want MT4129, got %s", hca.Hardware.HCAType)
			}
			if hca.Perf.OneWayBW != 360 {
				t.Errorf("HCAs[MT_0000000838].Perf.OneWayBW: want 360, got %v", hca.Perf.OneWayBW)
			}
		}
		break
	}
}
