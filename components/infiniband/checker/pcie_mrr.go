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
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
)

type PCIEMRRChecker struct {
	id          string
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewPCIEMRRChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &PCIEMRRChecker{
		id:   consts.CheckerIDInfinibandFW,
		name: config.CheckPCIEMRR,
		spec: specCfg,
	}, nil
}

func (c *PCIEMRRChecker) Name() string {
	return c.name
}

func (c *PCIEMRRChecker) Description() string {
	return c.description
}

func (c *PCIEMRRChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *PCIEMRRChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal

	if len(infinibandInfo.IBHardWareInfo) == 0 {
		result.Status = consts.StatusAbnormal
		result.Suggestion = ""
		result.Detail = config.NOIBFOUND
		return &result, fmt.Errorf("fail to get the IB device")
	}

	failedHcas := make([]string, 0)
	spec := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	curr := make([]string, 0, len(infinibandInfo.IBHardWareInfo))
	var faiedHcasSpec []string
	var faiedHcasCurr []string
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		if _, ok := c.spec.HCAs[hwInfo.BoardID]; !ok {
			log.Printf("HCA %s not found in spec, skipping %s", hwInfo.BoardID, c.name)
			continue
		}
		hcaSpec := c.spec.HCAs[hwInfo.BoardID]
		spec = append(spec, hcaSpec.Hardware.PCIEMRR)
		curr = append(curr, hwInfo.PCIEMRR)
		if hwInfo.PCIEMRR != hcaSpec.Hardware.PCIEMRR {
			result.Status = consts.StatusAbnormal
			failedHcas = append(failedHcas, hwInfo.IBDev)
			faiedHcasSpec = append(faiedHcasSpec, hcaSpec.Hardware.PCIEMRR)
			faiedHcasCurr = append(faiedHcasCurr, hwInfo.PCIEMRR)
			// auto fix if the curr not match the spec
			ModifyPCIeMaxReadRequest(hwInfo.PCIEBDF, "68", 5)
		}
	}

	result.Curr = strings.Join(curr, ",")
	result.Spec = strings.Join(spec, ",")
	result.Device = strings.Join(failedHcas, ",")
	if len(failedHcas) != 0 {
		result.Detail = fmt.Sprintf("PCIEMRR check fail: %s expect %s, but get %s", strings.Join(failedHcas, ","), faiedHcasSpec, faiedHcasCurr)
		result.Suggestion = fmt.Sprintf("Set %s with PCIe MaxReadReq %s", strings.Join(failedHcas, ","), strings.Join(faiedHcasSpec, ","))
	}

	return &result, nil
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

	// fmt.Printf("Current value: 0x%04X\n", currentValue)

	// Modify the high nibble
	// Clear the top 4 bits (0x0FFF mask)
	newValue := currentValue & 0x0FFF
	// Set the new high nibble
	newValue |= uint64(newHighNibble) << 12

	log.Printf("Modifying PCIe Max Read Request for device %s at offset %s: current value 0x%04X, new value 0x%04X", deviceAddr, offset, currentValue, newValue)
	// fmt.Printf("New value: 0x%04X\n", newValue)

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
