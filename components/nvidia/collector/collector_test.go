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
	"fmt"
	"os"
	"testing"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

var nvmlInst nvml.Interface

// setup function to initialize shared resources
func setup() error {
	// Initialize NVML
	nvmlInst = nvml.New()
	ret := nvmlInst.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
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

func TestSoftwareInfo_Get(t *testing.T) {
	// Create a SoftwareInfo instance
	softwareInfo := &SoftwareInfo{}

	// Call the Get method
	err := softwareInfo.Get()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the results
	expectedDriverVersion := "535.129.03" // Update with the expected value
	if softwareInfo.DriverVersion != expectedDriverVersion {
		t.Errorf("Incorrect driver version. Expected: %s, Got: %s", expectedDriverVersion, softwareInfo.DriverVersion)
	}

	// expectedCUDAVersion := 11 // Update with the expected value
	// if softwareInfo.CUDAVersion != expectedCUDAVersion {
	// 	t.Errorf("Incorrect CUDA version. Expected: %d, Got: %d", expectedCUDAVersion, softwareInfo.CUDAVersion)
	// }

	// expectedVBIOSVersion := "96.00.89.00.01" // Update with the expected value
	// if softwareInfo.VBIOSVersion != expectedVBIOSVersion {
	// 	t.Errorf("Incorrect VBIOS version. Expected: %s, Got: %s", expectedVBIOSVersion, softwareInfo.VBIOSVersion)
	// }

	// expectedFabricManagerVersion := "535.129.03" // Update with the expected value
	// if softwareInfo.FabricManagerVersion != expectedFabricManagerVersion {
	// 	t.Errorf("Incorrect Fabric Manager version. Expected: %s, Got: %s", expectedFabricManagerVersion, softwareInfo.FabricManagerVersion)
	// }

	t.Logf("SoftwareInfo: %+v", softwareInfo.ToString())
}

func TestPCIeInfo_Get(t *testing.T) {
	// Get the number of GPUs
	deviceCount, ret := nvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device count: %v", nvml.ErrorString(ret))
	}

	if deviceCount == 0 {
		t.Skip("No GPUs found")
	}
	device, ret := nvmlInst.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device handle for index 0: %v", nvml.ErrorString(ret))
	}

	// Create a PCIeInfo instance
	pcieInfo := &PCIeInfo{}

	// Call the Get method
	err := pcieInfo.Get(device, "0")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	t.Logf("PCIeInfo: %+v", pcieInfo.ToString())
}

func TestStatesInfo_Get(t *testing.T) {
	// Get the number of GPUs
	deviceCount, ret := nvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device count: %v", nvml.ErrorString(ret))
	}

	if deviceCount == 0 {
		t.Skip("No GPUs found")
	}
	device, ret := nvmlInst.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device handle for index 0: %v", nvml.ErrorString(ret))
	}

	// Create a StatesInfo instance
	statesInfo := &StatesInfo{}

	// Call the Get method
	err := statesInfo.Get(device, "0")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the results
	expectedPersistenced := "enable" // Update with the expected value
	if statesInfo.GpuPersistenced != expectedPersistenced {
		t.Errorf("Incorrect GPU persistence mode. Expected: %s, Got: %s", expectedPersistenced, statesInfo.GpuPersistenced)
	}

	expectedPstate := 0 // Update with the expected value
	if statesInfo.GpuPstate != uint32(expectedPstate) {
		t.Errorf("Incorrect GPU performance state. Expected: %d, Got: %d", expectedPstate, statesInfo.GpuPstate)
	}

	t.Logf("StatesInfo: %+v", statesInfo.ToString())
}

func TestNVLinkStates_Get(t *testing.T) {
	// Get the number of GPUs
	deviceCount, ret := nvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device count: %v", nvml.ErrorString(ret))
	}

	if deviceCount == 0 {
		t.Skip("No GPUs found")
	}
	device, ret := nvmlInst.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device handle for index 0: %v", nvml.ErrorString(ret))
	}

	// Create a mock NVLinkStates instance
	nvlinkStates := NVLinkStates{}

	// Call the Get method
	err := nvlinkStates.Get(device, "0")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	t.Logf("nvlinkStates: %+v", nvlinkStates.ToString())

}

func TestMemoryErrors_Get(t *testing.T) {
	// Get the number of GPUs
	deviceCount, ret := nvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device count: %v", nvml.ErrorString(ret))
	}

	if deviceCount == 0 {
		t.Skip("No GPUs found")
	}
	device, ret := nvmlInst.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device handle for index 0: %v", nvml.ErrorString(ret))
	}

	// Create a mock MemoryErrors instance
	memoryErrors := &MemoryErrors{}

	// Call the Get method
	memoryErrors.Get(device, "mock_uuid")

	t.Logf("MemoryErrors: %+v", memoryErrors.ToString())
}

func TestClockInfo_Get(t *testing.T) {
	// Get the number of GPUs
	deviceCount, ret := nvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device count: %v", nvml.ErrorString(ret))
	}

	if deviceCount == 0 {
		t.Skip("No GPUs found")
	}
	device, ret := nvmlInst.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device handle for index 0: %v", nvml.ErrorString(ret))
	}

	// Create a ClockEvents instance
	clockInfo := &ClockInfo{}

	// Call the Get method
	err := clockInfo.Get(device, "mock-uuid")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	t.Logf("clockInfo: %+v", clockInfo.ToString())
}

func TestClockEvents_Get(t *testing.T) {
	// Get the number of GPUs
	deviceCount, ret := nvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device count: %v", nvml.ErrorString(ret))
	}

	if deviceCount == 0 {
		t.Skip("No GPUs found")
	}
	device, ret := nvmlInst.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device handle for index 0: %v", nvml.ErrorString(ret))
	}

	// Create a ClockEvents instance
	clockEvents := &ClockEvents{}

	// Call the Get method
	err := clockEvents.Get(device, "mock-uuid")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	t.Logf("ClockEvents: %+v", clockEvents.ToString())
}

func TestDeviceInfo_Get(t *testing.T) {
	// Get the number of GPUs
	deviceCount, ret := nvmlInst.DeviceGetCount()
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device count: %v", nvml.ErrorString(ret))
	}

	if deviceCount == 0 {
		t.Skip("No GPUs found")
	}
	device, ret := nvmlInst.DeviceGetHandleByIndex(0)
	if ret != nvml.SUCCESS {
		t.Errorf("Failed to get device handle for index 0: %v", nvml.ErrorString(ret))
	}

	// Create a DeviceInfo instance
	deviceInfo := &DeviceInfo{}

	// Call the Get method
	err := deviceInfo.Get(device, 0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	t.Logf("DeviceInfo: %+v", deviceInfo.ToString())
}

func TestNvidiaInfo_Get(t *testing.T) {
	// Create a NvidiaInfo instance
	nvidiaInfo := &NvidiaInfo{}

	// Call the Get method
	err := nvidiaInfo.Get(nvmlInst)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the results
	expectedDeviceCount := 8 // Update with the expected value
	if nvidiaInfo.DeviceCount != expectedDeviceCount {
		t.Errorf("Incorrect device count. Expected: %d, Got: %d", expectedDeviceCount, nvidiaInfo.DeviceCount)
	}

	t.Logf("NvidiaInfo: %+v", nvidiaInfo.ToString())
}

func TestNvidiaCollector_Collect(t *testing.T) {
	// Create a NvidiaInfo instance
	NvidiaCollector, err := NewNvidiaCollector(nvmlInst, 8)
	if err != nil {
		t.Fatalf("Failed to create NvidiaCollector: %v", err)
	}

	// Call the Collect method
	nvidiaInfo, err := NvidiaCollector.Collect()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the results
	expectedDeviceCount := 8 // Update with the expected value
	if nvidiaInfo.DeviceCount != expectedDeviceCount {
		t.Errorf("Incorrect device count. Expected: %d, Got: %d", expectedDeviceCount, nvidiaInfo.DeviceCount)
	}

	t.Logf("NvidiaInfo: %+v", nvidiaInfo.ToString())
}
