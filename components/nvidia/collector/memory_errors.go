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

	"github.com/scitix/sichek/components/common"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type MemoryErrors struct {
	ECCMode      ECCMode      `json:"ecc_mode"`
	RemappedRows RemappedRows `json:"remapped_rows"`

	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1g08978d1c4fb52b6a4c72b39de144f1d9
	// Volatile counts are reset each time the driver loads.
	VolatileECC LocationErrors `json:"volatile"`
	// Aggregate counts persist across reboots (i.e. for the lifetime of the device).
	AggregateECC LocationErrors `json:"aggregate"`
}
type ECCMode struct {
	Current int32 `json:"Current"`
	Pending int32 `json:"Pending"`
}

type RemappedRows struct {
	RemappedDueToCorrectable   int `json:"remapped_due_to_correctable,omitempty"`
	RemappedDueToUncorrectable int `json:"remapped_due_to_uncorrectable,omitempty"`

	// Yes/No.
	// If uncorrectable error is >0, this pending field is set to "Yes".
	// For a100/h100, it requires a GPU reset to actually remap the row.
	// ref. https://docs.nvidia.com/deploy/a100-gpu-mem-error-mgmt/index.html#rma-policy-thresholds
	RemappingPending bool `json:"remapping_pending,omitempty"`

	// Yes/No
	RemappingFailureOccurred bool `json:"Remapping Failure Occurred,omitempty"`
}

type LocationErrors struct {
	// Total ECC error counts for the device.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g9748430b6aa6cdbb2349c5e835d70b0f
	Total ErrorType `json:"TOTAL"`

	// Memory locations types.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1g9bcbee49054a953d333d4aa11e8b9c25
	// L1Cache          ErrorType `json:"GPU L1 Cache"`
	// L2Cache          ErrorType `json:"GPU L2 Cache"`
	DRAM ErrorType `json:"Turing+ DRAM"`
	// GPUDeviceMemory  ErrorType `json:"GPU Device Memory"`
	// GPURegisterFile  ErrorType `json:"GPU Register File"`
	// GPUTextureMemory ErrorType `json:"GPU Texture Memory"`
	// SharedMemory     ErrorType `json:"Shared memory"`
	// CBU              ErrorType `json:"CBU"`
	SRAM ErrorType `json:"Turing+ SRAM"`
}

type ErrorType struct {
	// A memory error that was correctedFor ECC errors, these are single bit errors.
	// For Texture memory, these are errors fixed by resend.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1gc5469bd68b9fdcf78734471d86becb24
	Corrected uint64 `json:"corrected"`

	// A memory error that was not correctedFor ECC errors.
	// these are double bit errors For Texture memory.
	// these are errors where the resend fails.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1gc5469bd68b9fdcf78734471d86becb24
	Uncorrected uint64 `json:"uncorrected"`
}

func (memErrors *MemoryErrors) JSON() ([]byte, error) {
	return common.JSON(memErrors)
}

// Convert struct to JSON (pretty-printed)
func (memErrors *MemoryErrors) ToString() string {
	return common.ToString(memErrors)
}

func (memErrors *MemoryErrors) Get(device nvml.Device, uuid string) error {
	err := memErrors.getECCMode(device)
	if err != nil {
		return fmt.Errorf("failed to get ecc mode: %w", err)
	}
	err = memErrors.getRemappedRows(device, uuid)
	if err != nil {
		return fmt.Errorf("failed to get RemappedRows: %w", err)
	}
	err = memErrors.getECCErrors(device, uuid)
	if err != nil {
		return fmt.Errorf("failed to get ECCErrors: %w", err)
	}
	return nil
}

func (memErrors *MemoryErrors) getECCMode(device nvml.Device) error {
	current, pending, ret := device.GetEccMode()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to get current/pending ecc mode: %s", nvml.ErrorString(ret))
	}

	memErrors.ECCMode.Current = int32(current)
	memErrors.ECCMode.Pending = int32(pending)

	return nil
}

// Get Remapped Rows
func (memErrors *MemoryErrors) getRemappedRows(device nvml.Device, uuid string) error {
	remappedRowsCorrectable, remappedRowsUncorrectable, remappingPending, remappingFailureOccurred, err := device.GetRemappedRows()
	if err != nvml.SUCCESS {
		if err == nvml.ERROR_NOT_SUPPORTED {
			return nil
		}
		return fmt.Errorf("failed to get remapped rows for GPU %v due to : %v", uuid, err)
	} else {
		memErrors.RemappedRows.RemappedDueToCorrectable = remappedRowsCorrectable
		memErrors.RemappedRows.RemappedDueToUncorrectable = remappedRowsUncorrectable
		memErrors.RemappedRows.RemappingPending = remappingPending
		memErrors.RemappedRows.RemappingFailureOccurred = remappingFailureOccurred
	}
	return nil
}

func (memErrors *MemoryErrors) getECCErrors(device nvml.Device, uuid string) error {
	var err nvml.Return
	var result error
	memErrors.AggregateECC.Total.Corrected, err = device.GetTotalEccErrors(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.AGGREGATE_ECC,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(Total, Aggregate, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}
	memErrors.AggregateECC.Total.Uncorrected, err = device.GetTotalEccErrors(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(Total, Aggregate, uncorrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.Total.Corrected, err = device.GetTotalEccErrors(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.VOLATILE_ECC,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(Total, Volatile, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.Total.Uncorrected, err = device.GetTotalEccErrors(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(Total, Volatile, Uncorrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	// ECC mode is not enabled -- skipping fetching memory error counts using 'GetMemoryErrorCounter'
	if memErrors.ECCMode.Current != int32(nvml.FEATURE_ENABLED) {
		return nil
	}

	memErrors.AggregateECC.DRAM.Corrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_DRAM,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(DRAM, Aggregate, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.DRAM.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_DRAM,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(DRAM, Aggregate, Uncorrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.SRAM.Corrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_SRAM,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(SRAM, Volatile, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.SRAM.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_SRAM,
	)
	if err != nvml.SUCCESS && err != nvml.ERROR_NOT_SUPPORTED {
		result = fmt.Errorf("(SRAM, Volatile, UnCorrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}
	return result
}
