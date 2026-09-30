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
	"errors"
	"fmt"

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type PCIeInfo struct {
	BDFID           string `json:"bdf_id,omitempty" yaml:"bdf_id,omitempty"` // e.g 00000001:45:00.0
	DEVID           uint32 `json:"device_id" yaml:"device_id"`               // e.g 0x233010DE
	PCILinkGen      int    `json:"pci_gen" yaml:"pci_gen"`
	PCILinkGenMAX   int    `json:"pci_gen_max,omitempty" yaml:"pci_gen_max,omitempty"`
	PCILinkWidth    int    `json:"pci_width" yaml:"pci_width"`
	PCILinkWidthMAX int    `json:"pci_width_max,omitempty" yaml:"pci_width_max,omitempty"`
	PCIeTx          uint32 `json:"PCIeTx,omitempty" yaml:"PCIeTx,omitempty"`
	PCIeRx          uint32 `json:"PCIeRx,omitempty" yaml:"PCIeRx,omitempty"`
}

func (p *PCIeInfo) JSON() ([]byte, error) {
	return common.JSON(p)
}

// ToString Convert struct to JSON (pretty-printed)
func (p *PCIeInfo) ToString() string {
	return common.ToString(p)
}

func (p *PCIeInfo) Get(device nvml.Device, uuid string) error {

	// Get PCI Info
	pciInfo, ret := device.GetPciInfo()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get PCI info for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	p.BDFID = fmt.Sprintf("%04x:%02x:%02x.0", pciInfo.Domain, pciInfo.Bus, pciInfo.Device)
	p.DEVID = pciInfo.PciDeviceId

	// Get Current and Max PCIe Link Width
	pciLinkWidth, ret := device.GetCurrPcieLinkWidth()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current PCIe link width for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	p.PCILinkWidth = pciLinkWidth

	pciLinkWidthMax, ret := device.GetMaxPcieLinkWidth()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max PCIe link width for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	p.PCILinkWidthMAX = pciLinkWidthMax

	// Get Current and Max PCIe Link Generation
	pciLinkGen, ret := device.GetCurrPcieLinkGeneration()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current PCIe link generation for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	p.PCILinkGen = pciLinkGen

	pciLinkGenMax, ret := device.GetMaxPcieLinkGeneration()
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get max PCIe link generation for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	p.PCILinkGenMAX = pciLinkGenMax

	// Get PCIe Tx
	// Retrieve PCIe utilization information. This function is querying a byte counter over a 20ms interval and thus is the PCIe throughput over that interval.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries
	pcieTx, ret := device.GetPcieThroughput(nvml.PCIE_UTIL_TX_BYTES)
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get PCIe TxThroughput for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	p.PCIeTx = pcieTx

	// Get PCIe Rx
	pcieRx, ret := device.GetPcieThroughput(nvml.PCIE_UTIL_RX_BYTES)
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get PCIe RxThroughput for GPU %v: %v", uuid, nvml.ErrorString(ret))
	}
	p.PCIeRx = pcieRx

	return nil
}
