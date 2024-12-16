package utils

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestGetAllPCIeBDF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	result, err := GetAllPCIeBDF(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the result
	if len(result) == 0 {
		t.Errorf("Expected to find PCI devices, but got none")
	}

	t.Logf("%v", result)
}

func TestGetACSEnabledPCIEDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	// Call the function being tested
	acsEnDevices, err := GetACSEnabledPCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the result
	t.Logf("Got ACS enabled device BDF %v", acsEnDevices)
}

func TestGetACSCapPCIEDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	// Call the function being tested
	result, err := GetACSCapablePCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check the result
	for _, deviceBDF := range result {
		t.Logf("Got ACS Capable device BDF %s", deviceBDF)
	}
}

func TestACSEnableAndDisable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	acsCapDevices, err := GetACSCapablePCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(acsCapDevices) != 0 {
		// select the last device to test
		deviceBDF := acsCapDevices[len(acsCapDevices)-1]
		result := EnableACS(ctx, deviceBDF)
		if result != nil {
			t.Fatalf("Unexpected error: %v", result)
		}
		isACSDisable, _ := IsACSDisabled(ctx, deviceBDF)
		if isACSDisable {
			t.Fatalf("ACS is not enable for device %s", deviceBDF)
		}
		t.Logf("ACS enable for device %s successfully", deviceBDF)

		result = DisableACS(ctx, deviceBDF)
		if result != nil {
			t.Fatalf("Unexpected error: %v", result)
		}
		isACSDisable, err = IsACSDisabled(ctx, deviceBDF)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !isACSDisable {
			t.Fatalf("ACS is not disable for device %s", deviceBDF)
		}
		t.Logf("ACS disable for device %s successfully", deviceBDF)
	}
}

func TestDisableAllACS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	acsCapDevices, err := GetACSCapablePCIEDevices(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(acsCapDevices) != 0 {
		for _, deviceBDF := range acsCapDevices {
			result := EnableACS(ctx, deviceBDF)
			if result != nil {
				t.Fatalf("Unexpected error: %v", result)
			}
		}
		t.Logf("Enable ACS for all %d capable devices for test", len(acsCapDevices))

		pcieACS, err := IsAllACSDisabled(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if pcieACS == nil {
			t.Fatalf("Unexpected ACS is disabled for all devices")
		}

		pcieACS, err = DisableAllACS(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if pcieACS != nil {
			t.Fatalf("Unexpected ACS is not disabled for all devices")
		}
	}
}

func GetACSEnabledPCIEDevices(ctx context.Context) ([]string, error) {
	devices, _ := GetAllPCIeBDF(ctx)
	var acsDisabledDevices []string
	for _, deviceBDF := range devices {
		acsDisable, _ := IsACSDisabled(ctx, deviceBDF)
		if !acsDisable {
			acsDisabledDevices = append(acsDisabledDevices, deviceBDF)
		}
	}
	return acsDisabledDevices, nil
}

func GetACSCapablePCIEDevices(ctx context.Context) ([]string, error) {
	devices, _ := GetAllPCIeBDF(ctx)
	var acsCapDevices []string
	for _, deviceBDF := range devices {
		acsCap, err := ExecCommand(ctx, "setpci", "-s", deviceBDF, "ecap_acs+4.w")
		if err == nil {
			if strings.TrimSpace(string(acsCap))[0] != 'f' {
				acsCapDevices = append(acsCapDevices, deviceBDF)
			}
		}
	}
	return acsCapDevices, nil
}

func EnableACS(ctx context.Context, deviceBDF string) error {
	isACSDisable, _ := IsACSDisabled(ctx, deviceBDF)
	if isACSDisable {
		// Construct and run the setpci command
		_, err := ExecCommand(ctx, "setpci", "-v", "-s", deviceBDF, "ecap_acs+6.w=7f")
		if err != nil {
			logrus.WithField("component", "Utils").Errorf("Error enable ACS on device %v: %v", deviceBDF, err)
			return err
		}
		logrus.WithField("component", "Utils").Infof("Enabling ACS on device %v successfully", deviceBDF)
	}
	return nil
}
