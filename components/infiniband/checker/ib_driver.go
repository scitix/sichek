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
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type IBDriverChecker struct {
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBDriverChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBDriverChecker{
		name: config.CheckIBDriver,
		spec: specCfg,
	}, nil
}

func (c *IBDriverChecker) Name() string {
	return c.name
}

func (c *IBDriverChecker) Description() string {
	return c.description
}

func (c *IBDriverChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBDriverChecker) FindPCIDevicesByID(targetVendorID string, targetDeviceIDs []string) ([]string, error) {
	pciDevicesPath := "/sys/bus/pci/devices"
	bdfList := make([]string, 0)

	entries, err := os.ReadDir(pciDevicesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir '%s': %w", pciDevicesPath, err)
	}

	for _, entry := range entries {
		bdf := entry.Name()
		devPath := filepath.Join(pciDevicesPath, bdf)

		content, err := os.ReadFile(filepath.Join(devPath, "vendor"))
		if err != nil {
			return nil, fmt.Errorf("failed to read vendor file for %s: %w", bdf, err)
		}
		actualValue := strings.TrimSpace(string(content))
		actualValue = strings.TrimPrefix(actualValue, "0x")
		if strings.Compare(actualValue, targetVendorID) != 0 {
			logrus.WithField("component", "IBDriverChecker").Debugf("Skipping %s, vendor ID does not match: %s", bdf, targetVendorID)
			continue
		}

		for _, deviceID := range targetDeviceIDs {
			content, err = os.ReadFile(filepath.Join(devPath, "device"))
			if err != nil {
				return nil, fmt.Errorf("failed to read device file for %s: %w", bdf, err)
			}
			actualValue := strings.TrimSpace(string(content))
			actualValue = strings.TrimPrefix(actualValue, "0x")
			if strings.Compare(actualValue, deviceID) == 0 {
				bdfList = append(bdfList, bdf)
			}
		}
	}
	logrus.WithField("component", "infiniband").Infof("Found PCIe BDFs: %v", bdfList)

	return bdfList, nil
}

func (c *IBDriverChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	vendorID := "15b3"
	// TODO(wjli): config by specification
	deviceID := []string{"1021", "101b"}

	HCAPCIeBDF, err := c.FindPCIDevicesByID(vendorID, deviceID)
	if err != nil {
		log.Fatalf("fail to find PCI devices: %v", err)
	}

	var diffBDF []string
	for _, bdfFromPCIe := range HCAPCIeBDF {
		var diff bool = false

		for _, bdfFromDriver := range infinibandInfo.IBHardWareInfo {
			if strings.Compare(bdfFromPCIe, bdfFromDriver.PCIEBDF) == 0 {
				diff = false
				break
			} else {
				diff = true
			}
		}
		if diff {
			logrus.WithField("component", "infiniband").Infof("BDF from PCIe %s not found in IB hardware info", bdfFromPCIe)
			diffBDF = append(diffBDF, bdfFromPCIe)
		}
	}

	result := config.InfinibandCheckItems[c.name]
	if len(diffBDF) > 0 {
		result.Status = consts.StatusAbnormal
		result.Device = strings.Join(diffBDF, ",")
		result.Detail = fmt.Sprintf("Found PCIe BDFs not in IB hardware info: %v", diffBDF)
	} else {
		result.Status = consts.StatusNormal
		result.Detail = "All PCIe BDFs match IB hardware info"
	}
	return &result, nil
}
