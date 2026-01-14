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
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/caarlos0/log"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	PCIPath = "/sys/bus/pci/devices"
)

var (
	targetVendorID = []string{
		"0x15b3", // Mellanox Technologies
	}

	targetDeviceIDs = []string{
		"0x101b", // MT28908 Family [ConnectX-6]
		"0x101d", // MT28908 Family [ConnectX-6]
		"0x1021", // CMT2910 Family [ConnectX-7]
		"0x1023", // CX8 Family [ConnectX-8]
		"0xa2dc", // BlueField-3 E-series SuperNIC
		"0x09a2", // CMT2910 Family [ConnectX-7] HHHL
		"0x2330", // HPE/Enhance 400G
		"0x4128",
		"0x02b2",
	}
)

// FindIBPCIDevices finds IB PCI devices
func FindIBPCIDevices() (map[string]string, error) {
	log := logrus.WithField("component", "pci-scanner")

	if len(targetDeviceIDs) == 0 {
		log.Info("Target device ID list is empty, no devices can be matched.")
		return make(map[string]string), nil
	}

	if _, err := os.Stat(PCIPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("pci devices directory not found at %s: %w", PCIPath, err)
	}

	entries, err := os.ReadDir(PCIPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pci devices directory %s: %w", PCIPath, err)
	}

	log.Debugf("Scanning PCI devices in %s for vendor ID %s and device IDs %v", PCIPath, targetVendorID, targetDeviceIDs)

	foundDevices := make(map[string]string)

	for _, entry := range entries {
		pciAddr := entry.Name()
		deviceDir := filepath.Join(PCIPath, pciAddr)

		// Check if the device has IB/RoCE functionality by checking for infiniband directory
		infinibandPath := filepath.Join(deviceDir, "infiniband")
		if _, err := os.Stat(infinibandPath); os.IsNotExist(err) {
			// This is a management Ethernet card without IB/RoCE functionality, skip it
			log.Warnf("Skipping device %s: no infiniband directory found (management Ethernet card)", pciAddr)
			continue
		}

		// Read and compare vendor ID
		vendorBytes, err := os.ReadFile(filepath.Join(deviceDir, "vendor"))
		if err != nil {
			log.Warnf("Could not read vendor file for %s, skipping. Error: %v", pciAddr, err)
			continue
		}
		currentVendorID := strings.TrimSpace(string(vendorBytes))

		// If vendor ID doesn't match, skip this device directly
		if !slices.Contains(targetVendorID, currentVendorID) {
			continue
		}

		// Vendor ID matches, then read device ID
		deviceBytes, err := os.ReadFile(filepath.Join(deviceDir, "device"))
		if err != nil {
			log.Warnf("Could not read device file for %s, skipping. Error: %v", pciAddr, err)
			continue
		}
		currentDeviceID := strings.TrimSpace(string(deviceBytes))

		// Check if device ID is in the target list
		if slices.Contains(targetDeviceIDs, currentDeviceID) {
			log.Debugf("Found matching device: %s with vendor=%s, device=%s ", pciAddr, currentVendorID, currentDeviceID)
			foundDevices[pciAddr] = fmt.Sprintf("%s:%s", currentVendorID, currentDeviceID)
		}
	}

	log.Debugf("Finished PCI scan. Found %d matching devices.", len(foundDevices))
	return foundDevices, nil
}

// FindIBPCIDevices finds RDMA-capable PCI devices by checking infiniband sysfs
func GetRDMACapablePCIeDevices() (map[string]string, error) {
	if _, err := os.Stat(PCIPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("pci devices directory not found at %s: %w", PCIPath, err)
	}

	entries, err := os.ReadDir(PCIPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pci devices directory %s: %w", PCIPath, err)
	}

	foundDevices := make(map[string]string)

	for _, entry := range entries {
		pciAddr := entry.Name()
		deviceDir := filepath.Join(PCIPath, pciAddr)

		// Read vendor ID (optional, but strongly recommended to keep)
		vendorBytes, err := os.ReadFile(filepath.Join(deviceDir, "vendor"))
		if err != nil {
			logrus.WithField("component", "pci-scanner").Warnf("Could not read vendor file for %s, skipping. Error: %v", pciAddr, err)
			continue
		}
		currentVendorID := strings.TrimSpace(string(vendorBytes))

		if !slices.Contains(targetVendorID, currentVendorID) {
			logrus.WithField("component", "pci-scanner").Debugf("Skipping device %s: vendor %s not in target list", pciAddr, currentVendorID)
			continue
		}

		// device ID is only kept for information (not involved in the determination)
		deviceID := ""
		if deviceBytes, err := os.ReadFile(filepath.Join(deviceDir, "device")); err == nil {
			deviceID = strings.TrimSpace(string(deviceBytes))
		}

		// Check if the device has infiniband directory (core RDMA criterion)
		infinibandPath := filepath.Join(deviceDir, "infiniband")
		if _, err := os.Stat(infinibandPath); os.IsNotExist(err) {
			logrus.WithField("component", "pci-scanner").Warnf("Skipping device %s: no infiniband directory found (Maybe a management Ethernet card or IBLost)", pciAddr)
			continue
		}

		log.Debugf(
			"Found RDMA PCI device: %s (vendor=%s device=%s)",
			pciAddr, currentVendorID, deviceID,
		)

		foundDevices[pciAddr] = fmt.Sprintf("%s:%s", currentVendorID, deviceID)
	}

	log.Infof("Finished PCI scan. Found %d RDMA-capable devices.", len(foundDevices))
	return foundDevices, nil
}

type PCIETreeInfo struct {
	PCIETreeSpeed []PCIETreeSpeedInfo `json:"pcie_tree_speed" yaml:"pcie_tree_speed"`
	PCIETreeWidth []PCIETreeWidthInfo `json:"pcie_tree_width" yaml:"pcie_tree_width"`
}

type PCIETreeSpeedInfo struct {
	BDF   string `json:"bdf" yaml:"bdf"`
	Speed string `json:"speed" yaml:"speed"`
}

type PCIETreeWidthInfo struct {
	BDF   string `json:"bdf" yaml:"bdf"`
	Width string `json:"width" yaml:"width"`
}

// Collect collects PCIe tree information for a given IB device
func (pcie *PCIETreeInfo) Collect(IBDev string) {
	pcie.PCIETreeSpeed = GetPCIETreeSpeed(IBDev)
	pcie.PCIETreeWidth = pcie.GetPCIETreeWidth(IBDev)
}

// GetPCIETreeWidth gets PCIe tree width information
func (c *PCIETreeInfo) GetPCIETreeWidth(IBDev string) []PCIETreeWidthInfo {
	bdf := GetIBDevBDF(IBDev)
	if len(bdf) == 0 {
		return nil
	}
	devicePath := filepath.Join(PCIPath, bdf[0])
	cmd := exec.Command("readlink", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	bdfRegexPattern := `\b[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]\b`
	re := regexp.MustCompile(bdfRegexPattern)
	bdfs := re.FindAllString(string(output), -1)
	allTreeWidth := make([]PCIETreeWidthInfo, 0, len(bdfs))

	for _, bdf := range bdfs {
		var perTreeWidth PCIETreeWidthInfo
		width, err := GetFileCnt(filepath.Join(PCIPath, bdf, "current_link_width"))
		if err != nil || len(width) == 0 {
			logrus.WithField("component", "infiniband").Warnf("Failed to read PCIe width for BDF %s: %v", bdf, err)
			continue
		}
		logrus.WithField("component", "infiniband").Infof("get the pcie tree width, ib:%s bdf:%s width:%s", IBDev, bdf, width[0])
		perTreeWidth.BDF = bdf
		perTreeWidth.Width = width[0]
		allTreeWidth = append(allTreeWidth, perTreeWidth)
	}
	return allTreeWidth
}

// GetPCIECLinkSpeed gets PCIe current link speed
func GetPCIECLinkSpeed(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "device/current_link_speed")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read PCIe link speed for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// GetPCIECLinkWidth gets PCIe current link width
func GetPCIECLinkWidth(IBDev string) string {
	result, err := ReadIBDevSysfileLines(IBDev, "device/current_link_width")
	if err != nil || len(result) == 0 {
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("Failed to read PCIe link width for %s: %v", IBDev, err)
		}
		return ""
	}
	return result[0]
}

// GetPCIEMRR gets PCIe Max Read Request
func GetPCIEMRR(ctx context.Context, IBDev string) []string {
	bdf := GetIBDevBDF(IBDev)
	if len(bdf) == 0 {
		return nil
	}

	// lspciCmd := exec.Command("lspci", "-s", bdf[0], "-vvv")
	lspciOutput, err := utils.ExecCommand(ctx, "lspci", "-s", bdf[0], "-vvv")
	if err != nil {
		return nil
	}

	grepCmd := exec.Command("grep", "MaxReadReq")
	grepCmd.Stdin = bytes.NewBuffer(lspciOutput)
	grepOutput, err := grepCmd.Output()
	if err != nil {
		return nil
	}

	parts := strings.Split(string(grepOutput), "MaxReadReq ")
	var mrr []string
	if len(parts) > 1 {
		mrr = strings.Fields(parts[1])
		// autofix
		if strings.Compare(mrr[0], "4096") != 0 {
			// get BDF
			bdf := GetIBDevBDF(IBDev)
			if len(bdf) > 0 {
				// autofix
				if err := ModifyPCIeMaxReadRequest(bdf[0], "68", 5); err != nil {
					logrus.WithField("component", "infiniband").Errorf("Failed to modify PCIe Max Read Request for %s: %v", bdf[0], err)
				}
			}
		}
	}

	return mrr
}

// GetPCIETreeMin gets minimum PCIe tree value for a given link type
func GetPCIETreeMin(IBDev, linkType string) string {
	bdfList := GetIBDevBDF(IBDev)
	if len(bdfList) == 0 {
		logrus.WithField("component", "infiniband").Warnf("Could not get BDF for IB device %s", IBDev)
		return ""
	}
	// bdf is the BDF address of the terminal device itself
	bdf := bdfList[0]

	devicePath := filepath.Join(PCIPath, bdf)
	linkPath, err := os.Readlink(devicePath)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("Failed to resolve symlink for %s: %v", devicePath, err)
		return ""
	}

	bdfRegexPattern := `\b[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]\b`
	re := regexp.MustCompile(bdfRegexPattern)

	allBdfsInPath := re.FindAllString(string(linkPath), -1)

	// Filter out the device's own BDF
	var upstreamBdfs []string
	for _, foundBdf := range allBdfsInPath {
		if foundBdf != bdf {
			upstreamBdfs = append(upstreamBdfs, foundBdf)
		}
	}

	if len(upstreamBdfs) == 0 {
		// If there are no upstream devices (e.g., device directly connected to CPU), this is normal, just return
		logrus.WithField("component", "infiniband").Infof("No upstream PCIe devices found in path for %s, skipping check.", bdf)
		return ""
	}

	if len(upstreamBdfs) == 1 {
		// If there's only one upstream device, it means it's directly connected to CPU, this is also normal, just return
		logrus.WithField("component", "infiniband").Infof("Only one upstream PCIe device found in path for %s, likely direct to CPU, skipping check.", bdf)
		return ""
	}

	logrus.WithField("component", "infiniband").Infof("Checking upstream devices for %s: %v", bdf, upstreamBdfs)

	var minNumericString string
	minNumericValue := math.MaxFloat64

	// Now, we only iterate through the BDF list of upstream devices
	for _, currentBdf := range upstreamBdfs {
		logrus.WithField("component", "infiniband").Infof("Checking upstream device %s for property %s", currentBdf, linkType)
		propertyFilePath := filepath.Join(PCIPath, currentBdf, linkType)

		propertyContents, err := GetFileCnt(propertyFilePath)
		if err != nil || len(propertyContents) == 0 {
			if err != nil {
				logrus.WithField("component", "infiniband").Debugf("Property file '%s' is unreadable for BDF %s: %v, skipping.", linkType, currentBdf, err)
			} else {
				logrus.WithField("component", "infiniband").Debugf("Property file '%s' is empty for BDF %s, skipping.", linkType, currentBdf)
			}
			continue
		}

		currentPropertyString := strings.TrimSpace(propertyContents[0])
		parts := strings.Fields(currentPropertyString)
		if len(parts) == 0 {
			logrus.WithField("component", "infiniband").Warnf("Malformed property string '%s' for BDF %s", currentPropertyString, currentBdf)
			continue
		}
		numericStringPart := parts[0]

		currentNumericValue, err := strconv.ParseFloat(numericStringPart, 64)
		if err != nil {
			logrus.WithField("component", "infiniband").Warnf("Could not parse numeric value from '%s' in file %s", numericStringPart, propertyFilePath)
			continue
		}

		if currentNumericValue < minNumericValue {
			minNumericValue = currentNumericValue
			minNumericString = numericStringPart
			logrus.WithField("component", "infiniband").Debugf(
				"Found new upstream minimum for %s (%s): %s (full value: '%s', at BDF: %s)",
				IBDev, linkType, minNumericString, currentPropertyString, currentBdf,
			)
		}
	}

	if minNumericString == "" {
		logrus.WithField("component", "infiniband").Warnf("Could not determine a minimum value for property '%s' on upstream path of device %s", linkType, IBDev)
	}

	return minNumericString
}

// GetPCIETreeSpeed gets PCIe tree speed information
func GetPCIETreeSpeed(IBDev string) []PCIETreeSpeedInfo {
	bdf := GetIBDevBDF(IBDev)
	if len(bdf) == 0 {
		return nil
	}
	devicePath := filepath.Join(PCIPath, bdf[0])
	cmd := exec.Command("readlink", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	bdfRegexPattern := `\b[0-9a-fA-F]{4}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}\.[0-7]\b`
	re := regexp.MustCompile(bdfRegexPattern)
	bdfs := re.FindAllString(string(output), -1)
	allTreeSpeed := make([]PCIETreeSpeedInfo, 0, len(bdfs))
	logrus.WithField("component", "infiniband").Infof("get the pcie tree speed, ib:%s bdfs:%v", IBDev, bdfs)

	for _, bdf := range bdfs {
		var perTreeSpeed PCIETreeSpeedInfo
		speed, err := GetFileCnt(filepath.Join(PCIPath, bdf, "current_link_speed"))
		if err != nil || len(speed) == 0 {
			logrus.WithField("component", "infiniband").Warnf("Failed to read PCIe speed for BDF %s: %v", bdf, err)
			continue
		}
		logrus.WithField("component", "infiniband").Infof("get the pcie tree speed, ib:%s bdf:%s speed:%s", IBDev, bdf, speed[0])
		perTreeSpeed.BDF = bdf
		perTreeSpeed.Speed = speed[0]
		allTreeSpeed = append(allTreeSpeed, perTreeSpeed)
	}
	return allTreeSpeed
}

// ModifyPCIeMaxReadRequest modifies the Max Read Request Size of a PCIe device
// deviceAddr: PCI device address, e.g., "80:00.0"
// offset: Register offset address, e.g., "68"
// newHighNibble: New high nibble value (0-F)
func ModifyPCIeMaxReadRequest(deviceAddr string, offset string, newHighNibble int) error {
	// Validate input parameters
	if newHighNibble < 0 || newHighNibble > 0xF {
		return fmt.Errorf("new high nibble value must be between 0-F")
	}

	// Read current value
	readCmd := exec.Command("setpci", "-s", deviceAddr, offset+".w")
	output, err := readCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to read PCI register: %v", err)
	}

	// Parse the returned hexadecimal value
	currentValueStr := strings.TrimSpace(string(output))
	currentValue, err := strconv.ParseUint(currentValueStr, 16, 16)
	if err != nil {
		return fmt.Errorf("failed to parse hex value: %v", err)
	}

	// Modify the high nibble
	// Clear the top 4 bits (0x0FFF mask)
	newValue := currentValue & 0x0FFF
	// Set the new high nibble
	newValue |= uint64(newHighNibble) << 12

	logrus.WithField("component", "infiniband").Infof("Modifying PCIe Max Read Request for device %s at offset %s: current value 0x%04X, new value 0x%04X", deviceAddr, offset, currentValue, newValue)

	// Write back the new value
	writeValueStr := fmt.Sprintf("%04x", newValue)
	writeCmd := exec.Command("setpci", "-s", deviceAddr, offset+".w="+writeValueStr)
	err = writeCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to write PCI register: %v", err)
	}

	// Verify the write was successful
	verifyCmd := exec.Command("setpci", "-s", deviceAddr, offset+".w")
	verifyOutput, err := verifyCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to verify write result: %v", err)
	}

	verifiedValueStr := strings.TrimSpace(string(verifyOutput))
	verifiedValue, err := strconv.ParseUint(verifiedValueStr, 16, 16)
	if err != nil {
		return fmt.Errorf("failed to parse verification value: %v", err)
	}

	if verifiedValue != newValue {
		return fmt.Errorf("write verification failed: expected 0x%04X, got 0x%04X", newValue, verifiedValue)
	}

	fmt.Printf("Successfully modified! Verified value: 0x%04X\n", verifiedValue)
	return nil
}
