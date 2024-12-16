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
package checker

import (
	"context"
	"fmt"
	"os"
	"testing"

	sram "github.com/scitix/sichek/components/nvidia/checker/check_ecc_sram"
	remap "github.com/scitix/sichek/components/nvidia/checker/check_remmaped_rows"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// define the shared NvidiaInfo
var nvidiaInfo *collector.NvidiaInfo
var nvmlInst nvml.Interface
var cfg config.NvidiaConfig

// setup function to initialize shared resources
func setup() error {
	// Initialize NVML
	nvmlInst = nvml.New()
	ret := nvmlInst.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
	}
	// Call the Get method
	nvidiaCollector, err := collector.NewNvidiaCollector(nvmlInst, 8)
	if err != nil {
		return fmt.Errorf("failed to create NvidiaCollector: %v", err)
	}
	nvidiaInfo, err = nvidiaCollector.Collect()
	if err != nil {
		return fmt.Errorf("unexpected error: %v", err)
	}

	cfg = config.NvidiaConfig{}
	deviceId := nvidiaInfo.DevicesInfo[0].PCIeInfo.DEVID
	deviceID := fmt.Sprintf("0x%x", deviceId)
	specFile, err := config.GetSpec(deviceID)
	if err != nil {
		panic("failed to get spec file for NVIDIA GPU")
	}
	err = cfg.LoadFromYaml("", specFile)
	if err != nil {
		fmt.Printf("NewNvidia load config yaml %s failed: %v", specFile, err)
		return err
	}

	return nil
}

// teardown function to clean up shared resources
func teardown() {
	if nvmlInst != nil {
		fmt.Println("Shutting down NVML")
		nvmlInst.Shutdown()
	}
}

// TestMain is the entry point for testing
func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		fmt.Printf("setup failed: %v", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Call teardown after running tests
	teardown()

	// Exit with the code from m.Run()
	os.Exit(code)
}

func TestAppClocksChecker_Check(t *testing.T) {
	// Create a new AppClocksChecker
	checker, err := NewAppClocksChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create AppClocksChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestClockEventsChecker_Check(t *testing.T) {
	// Create a new ClockEventsChecker
	checker, err := NewClockEventsChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create ClockEventsChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestNvlinkChecker_Check(t *testing.T) {
	// Create a new NvlinkChecker
	checker, err := NewNvlinkChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create NvlinkChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestGpuPersistenceChecker_Check(t *testing.T) {
	// Create a new GpuPersistenceChecker
	checker, err := NewGpuPersistenceChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create GpuPersistenceChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestGpuPStateChecker_Check(t *testing.T) {
	// Create a new GpuPStateChecker
	checker, err := NewGpuPStateChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create GpuPStateChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestHardwareChecker_Check(t *testing.T) {
	// Create a new HardwareChecker
	checker, err := NewHardwareChecker(&cfg.Spec, nvmlInst)
	if err != nil {
		t.Fatalf("failed to create HardwareChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestRemmapedRowsFailureChecker_Check(t *testing.T) {
	// Create a new RemmapedRowsFailureChecker
	checker, err := remap.NewRemmapedRowsFailureChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create RemmapedRowsFailureChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestRemmapedRowsPendingChecker_Check(t *testing.T) {
	// Create a new RemmapedRowsPendingChecker
	checker, err := remap.NewRemmapedRowsPendingChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create RemmapedRowsPendingChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestRemmapedRowsUncorrectableChecker_Check(t *testing.T) {
	// Create a new RemmapedRowsUncorrectableChecker
	checker, err := remap.NewRemmapedRowsUncorrectableChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create RemmapedRowsUncorrectableChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestSRAMAggUncorrectableChecker_Check(t *testing.T) {
	// Create a new SRAMAggUncorrectableChecker
	checker, err := sram.NewSRAMAggUncorrectableChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create SRAMAggUncorrectableChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestSRAMHighcorrectableChecker_Check(t *testing.T) {
	// Create a new SRAMHighcorrectableChecker
	checker, err := sram.NewSRAMHighcorrectableChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create SRAMHighcorrectableChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestSRAMVolatileUncorrectableChecker_Check(t *testing.T) {
	// Create a new SRAMVolatileUncorrectableChecker
	checker, err := sram.NewSRAMVolatileUncorrectableChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create SRAMVolatileUncorrectableChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestPCIeChecker_Check(t *testing.T) {
	// Create a new PCIeChecker
	checker, err := NewPCIeChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create PCIeChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestSoftwareChecker_Check(t *testing.T) {
	// Create a new SoftwareChecker
	checker, err := NewSoftwareChecker(&cfg.Spec)
	if err != nil {
		t.Fatalf("failed to create SoftwareChecker: %v", err)
	}

	// Run the Check method
	result, err := checker.Check(context.Background(), nvidiaInfo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "normal" {
		t.Errorf("expected status 'normal', got %v", result.Status)
	}
}

func TestChecker_Check(t *testing.T) {
	// Create a new SoftwareChecker
	checkers, err := NewCheckers(&cfg, nvmlInst)
	if err != nil {
		t.Fatalf("failed to create Checkers: %v", err)
	}

	// Run the Check method
	for _, checker := range checkers {
		result, err := checker.Check(context.Background(), nvidiaInfo)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Status != "normal" {
			t.Errorf("expected status 'normal', got %v", result.Status)
		}
	}
}
