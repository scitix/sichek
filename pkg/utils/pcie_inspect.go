package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

type PCIeACS struct {
	BDF       string
	ACSStatus string
}

func GetAllPCIeBDF(ctx context.Context) ([]string, error) {
	devices, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return nil, fmt.Errorf("failed to list PCI devices: %w", err)
	}

	deviceBDFs := make([]string, 0, len(devices))
	for _, device := range devices {
		deviceBDF := device.Name()
		deviceBDFs = append(deviceBDFs, deviceBDF)
	}

	return deviceBDFs, nil
}

func GetACSStatus(ctx context.Context, BDF string) (string, error) {
	acsCtl, err := ExecCommand(ctx, "setpci", "-s", BDF, "ecap_acs+6.w")
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %w", err)
	}
	return strings.TrimSpace(string(acsCtl)), nil
}

func IsACSDisabled(ctx context.Context, BDF string) (bool, error) {
	acsCtl, err := ExecCommand(ctx, "setpci", "-s", BDF, "ecap_acs+6.w")
	if err != nil {
		return false, fmt.Errorf("failed to execute command: %w", err)
	}
	acsStatus := strings.TrimSpace(string(acsCtl))
	return acsStatus == "0000", nil
}

func IsAllACSDisabled(ctx context.Context) ([]PCIeACS, error) {
	var allDisabled []PCIeACS
	BDFs, err := GetAllPCIeBDF(ctx)
	if err != nil {
		logrus.WithField("component", "Utils").Errorf("Failed to get all PCIe BDFs: %v", err)
		return nil, fmt.Errorf("failed to get all PCIe BDFs: %w", err)
	}
	for _, BDF := range BDFs {
		acsDisabled, err := IsACSDisabled(ctx, BDF)
		// fail to run the cmd
		if err != nil {
			continue
		}
		// success run the cmd,but get unexpected result
		if !acsDisabled {
			logrus.WithField("component", "Utils").Warnf("ACS not disabled for PCIe device BDF: %s", BDF)

			var perDisabled PCIeACS
			status, err := GetACSStatus(ctx, BDF)
			if err != nil {
				logrus.WithField("component", "Utils").Warnf("Fail to get the PCIe status: %v", err)
			}
			perDisabled.BDF = BDF
			perDisabled.ACSStatus = status
			allDisabled = append(allDisabled, perDisabled)
		}
	}
	if len(allDisabled) > 0 {
		return allDisabled, fmt.Errorf("not all PCIe devices are enabled when check acs")
	}
	logrus.WithField("component", "Utils").Info("ACS is disabled on all PCIe devices")
	return nil, nil
}

func DisableACS(ctx context.Context, BDF string) error {
	acsDisabled, err := IsACSDisabled(ctx, BDF)
	if err != nil {
		// logrus.WithField("component", "Utils").Errorf("Failed to check ACS status for device %s: %v", BDF, err)
		return fmt.Errorf("failed to check ACS status for device %s: %w", BDF, err)
	}
	if !acsDisabled {
		output, err := ExecCommand(ctx, "setpci", "-v", "-s", BDF, "ecap_acs+6.w=0")
		if err != nil {
			logrus.WithField("component", "Utils").Errorf("Error disabling ACS on device %s: %v", BDF, err)
			return fmt.Errorf("error disabling ACS on device %s: %w", BDF, err)
		}
		logrus.WithField("component", "Utils").Infof("Disabled ACS on device %s successfully. Output: %s", BDF, output)
	} else {
		logrus.WithField("component", "Utils").Infof("ACS already disabled on device %s", BDF)
	}
	return nil
}

func DisableAllACS(ctx context.Context) ([]PCIeACS, error) {
	BDFs, err := GetAllPCIeBDF(ctx)
	if err != nil {
		logrus.WithField("component", "Utils").Errorf("Failed to get all PCIe BDFs: %v", err)
		return nil, fmt.Errorf("failed to get all PCIe BDFs: %w", err)
	}
	var failedDevices []PCIeACS
	for _, BDF := range BDFs {
		if err := DisableACS(ctx, BDF); err != nil {
			logrus.WithField("component", "Utils").Errorf("Failed to disable ACS on device %s: %v", BDF, err)

			status, err := GetACSStatus(ctx, BDF)
			if err != nil {
				logrus.WithField("component", "Utils").Warnf("Fail to get the PCIe status: %v", err)
			}
			failedDevices = append(failedDevices, PCIeACS{
				BDF:       BDF,
				ACSStatus: status,
			})
		}
	}
	if len(failedDevices) > 0 {
		return failedDevices, fmt.Errorf("not all PCIe devices are enabled when disable all acs")
	}

	logrus.WithField("component", "Utils").Info("Successfully disabled ACS on all PCIe devices")
	return nil, nil
}
