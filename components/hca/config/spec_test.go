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
	"fmt"
	"os"
	"testing"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

func TestLoadSpecFromFile(t *testing.T) {
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

	// Write sample spec data
	specData := `
nvidia:
  "0x233010de":
    name: NVIDIA H100 80GB HBM3
    gpu_nums: 8
    gpu_memory: 80
    pcie:
      pci_gen: 5
      pci_width: 16
hca:
  test_from_file:
    hardware:
      hca_type: "MT4129"
      board_id: "test_from_file"
      fw_ver: "28.39.2048"
      vpd: "P45645-001 / HPE InfiniBand NDR 1-port OSFP PCIe5 x16 MCX75310AAS-NEAT Adapter"
      net_port: 1
      port_speed: "400 Gb/sec (4X NDR)"
      phy_state: "LinkUp"
      port_state: "ACTIVE"
      net_operstate: "down"
      link_layer: "InfiniBand"
      pcie_width: "16"
      pcie_speed: "32.0 GT/s PCIe"
      pcie_tree_width: "32"
      pcie_tree_speed: "16"
      pcie_acs: "disable"
      pcie_mrr: "4096"
    perf:
      one_way_bw: 360 # Gbps
      avg_latency_us: 1.0 # us
`
	if _, err := specFile.Write([]byte(specData)); err != nil {
		t.Fatalf("Failed to write to temp spec file: %v", err)
	}

	// Test loading spec
	hcaSpecs := &HCASpecs{}
	err = hcaSpecs.tryLoadFromFile(specFile.Name())
	if err != nil {
		t.Fatalf("hcaSpecs.tryLoadFromFile() returned an error: %v", err)
	}

	// Validate loaded spec
	if _, ok := hcaSpecs.HcaSpec["test_from_file"]; !ok {
		t.Fatalf("Expected hardware key 'test_from_file', not found")
	}
}

func TestLoadSpecFromDevDefaultFile(t *testing.T) {
	hcaSpec := &HCASpecs{}
	err := hcaSpec.tryLoadFromDevConfig()
	if err != nil {
		t.Fatalf("TestLoadSpecFromDevDefaultFile() returned an error: %v", err)
	}

	// Validate loaded spec
	if spec, ok := hcaSpec.HcaSpec["MT_0000000970"]; !ok {
		t.Fatalf("Expected hardware key 'MT_0000000971', not found")
	} else {
		if spec.Hardware.BoardID != "MT_0000000970" {
			t.Fatalf("Expected BoardID 'MT_0000000970', got '%s'", spec.Hardware.BoardID)
		}
		if spec.Hardware.FWVer != "28.39.2048" {
			t.Fatalf("Expected FwVer '28.39.2048', got '%s'", spec.Hardware.FWVer)
		}
		if spec.Hardware.PCIEWidth != "16" {
			t.Fatalf("Expected PcieWidth '16', got '%s'", spec.Hardware.PCIEWidth)
		}
		if spec.Hardware.PCIESpeed != "32.0 GT/s PCIe" {
			t.Fatalf("Expected PcieSpeed '32.0 GT/s PCIe', got '%s'", spec.Hardware.PCIESpeed)
		}
		if spec.Perf.OneWayBW != 360 {
			t.Fatalf("Expected OneWayBW '360', got '%f'", spec.Perf.OneWayBW)
		}
		if spec.Perf.AvgLatency != 1.0 {
			t.Fatalf("Expected AvgLatency '1.0', got '%f'", spec.Perf.AvgLatency)
		}
	}
	if _, ok := hcaSpec.HcaSpec["MT_0000000971"]; !ok {
		t.Fatalf("Expected hardware key 'MT_0000000971', not found")
	}
	if _, ok := hcaSpec.HcaSpec["MT_0000001119"]; !ok {
		t.Fatalf("Expected hardware key 'MT_0000001119', not found")
	}
}

func TestLoadSpecFromOss(t *testing.T) {
	hcaSpec := &HCASpecs{}
	ibDevBoardId := "test"
	url := fmt.Sprintf("%s/%s/%s.yaml", consts.DefaultOssCfgPath, consts.ComponentNameHCA, ibDevBoardId)
	err := common.LoadSpecFromOss(url, hcaSpec)
	if err != nil {
		t.Fatalf("LoadSpecFromOss() returned an error: %v", err)
	}
	if len(hcaSpec.HcaSpec) == 0 {
		t.Fatalf("Expected HCAHardwares to be loaded, got empty map")
	}
	if _, ok := hcaSpec.HcaSpec[ibDevBoardId]; !ok {
		t.Fatalf("Expected hardware key '%s', not found", ibDevBoardId)
	}
	if hcaSpec.HcaSpec[ibDevBoardId].Hardware.BoardID != ibDevBoardId {
		t.Fatalf("Expected BoardID '%s', got '%s'", ibDevBoardId, hcaSpec.HcaSpec[ibDevBoardId].Hardware.BoardID)
	}
}

func TestGetBoardIDs(t *testing.T) {
	_, boardIDs, err := GetIBBoardIDs()
	if err != nil {
		t.Fatalf("getBoardIDs() returned an error: %v", err)
	}

	if len(boardIDs) == 0 {
		t.Fatal("Expected non-empty board IDs, got empty slice")
	}

	// Check if the first board ID is a non-empty string
	t.Logf("Get Board IDs: %v", boardIDs)
}

func TestLoadSpec(t *testing.T) {
	_, boardIDs, _ := GetIBBoardIDs()
	if len(boardIDs) == 1 && boardIDs[0] == "MT_0000000970" {
		hcaSpec, err := LoadSpec("")
		if err != nil {
			t.Fatalf("LoadSpec() returned an error: %v", err)
		}
		if hcaSpec == nil {
			t.Fatal("Expected non-nil HCA spec, got nil")
		}
		if _, ok := hcaSpec.HcaSpec[boardIDs[0]]; !ok {
			t.Fatalf("Expected hardware key '%s', not found", boardIDs[0])
		}
	} else {
		t.Skip("No valid board IDs found, skipping test")
	}
}
