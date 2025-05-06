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
package utils

import (
	"context"
	"testing"
	"time"
)

func TestGetAllPCIeBDF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := GetAllPCIeBDF(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the result
	if len(result) == 0 {
		t.Errorf("Expected to find PCI devices, but got none")
	}

	for _, deviceBDF := range result {
		t.Logf("%v", deviceBDF)
	}
	t.Logf("Got %d PCIe devices", len(result))

}

func TestGetACSCapDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Call the function being tested
	result, err := GetACSCapablePCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the result
	for _, deviceBDF := range result {
		t.Logf("Got %d ACS Capable device BDF %s", len(result), deviceBDF)
	}
}

func TestGetACSEnabledDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Call the function being tested
	acsEnDevices, err := GetACSEnabledDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the result
	for _, devicePCIACS := range acsEnDevices {
		t.Logf("Got %d ACS Enabled device BDF %s", len(acsEnDevices), devicePCIACS.BDF)
	}
}

func TestACSEnableAndDisable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	acsCapDevices, err := GetACSCapablePCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(acsCapDevices) != 0 {
		// select the last device to test
		t.Logf("======test: `EnableACS`=====")
		deviceBDF := acsCapDevices[len(acsCapDevices)-1]
		result := EnableACS(ctx, deviceBDF)
		if result != nil {
			t.Fatalf("Unexpected error: %v", result)
		}
		isACSDisable, _, _ := IsACSDisabled(ctx, deviceBDF)
		if isACSDisable {
			t.Fatalf("ACS is not enable for device %s", deviceBDF)
		}
		t.Logf("ACS is enabled for device %s successfully", deviceBDF)

		t.Logf("======test: `DisableACS`=====")
		result = DisableACS(ctx, deviceBDF)
		if result != nil {
			t.Fatalf("Unexpected error: %v", result)
		}
		isACSDisable, _, err = IsACSDisabled(ctx, deviceBDF)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !isACSDisable {
			t.Fatalf("ACS is not disable for device %s", deviceBDF)
		}
		t.Logf("ACS is disabled for device %s successfully", deviceBDF)
	}
}

func TestDisableBatchACS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	acsCapDevices, err := GetACSCapablePCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(acsCapDevices) != 0 {
		t.Logf("======test: `EnableACS for 2 capable devices`=====")
		batch := 0
		for _, deviceBDF := range acsCapDevices {
			batch++
			result := EnableACS(ctx, deviceBDF)
			if result != nil {
				t.Fatalf("Unexpected error: %v", result)
			}
			if batch == 2 {
				break
			}
		}
		t.Logf("Enable ACS for 2 capable devices for test")

		t.Logf("======test: `GetACSEnabledDevices`=====")
		pcieACS, err := GetACSEnabledDevices(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		t.Logf("Get %d enabled ACS devices", len(pcieACS))
		if len(pcieACS) != 2 {
			t.Fatalf("Got %d devices that ACS is enabled, expected 2", len(pcieACS))
		}

		t.Logf("======test: `BatchDisableACS`=====")
		pcieACS, err = BatchDisableACS(ctx, pcieACS)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pcieACS != nil {
			t.Fatalf("Unexpected ACS is not disabled for all devices")
		}
	}
}