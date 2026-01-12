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
	"github.com/sirupsen/logrus"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type MemoryErrors struct {
	ECCMode      ECCMode      `json:"ecc_mode" yaml:"ecc_mode"`
	RemappedRows RemappedRows `json:"remapped_rows" yaml:"remapped_rows"`

	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1g08978d1c4fb52b6a4c72b39de144f1d9
	// Volatile counts are reset each time the driver loads.
	VolatileECC      LocationErrors `json:"volatile" yaml:"volatile"`
	TotalVolatileECC uint64         `json:"total_volatile_ecc" yaml:"total_volatile_ecc"`
	// Aggregate counts persist across reboots (i.e. for the lifetime of the device).
	AggregateECC      LocationErrors `json:"aggregate" yaml:"aggregate"`
	TotalAggregateECC uint64         `json:"total_aggregate_ecc" yaml:"total_aggregate_ecc"`
}
type ECCMode struct {
	Current int32 `json:"Current" yaml:"Current"`
	Pending int32 `json:"Pending" yaml:"Pending"`
}

type RemappedRows struct {
	RemappedDueToCorrectable   int `json:"remapped_due_to_correctable,omitempty" yaml:"remapped_due_to_correctable,omitempty"`
	RemappedDueToUncorrectable int `json:"remapped_due_to_uncorrectable,omitempty" yaml:"remapped_due_to_uncorrectable,omitempty"`

	// Yes/No.
	// If uncorrectable error is >0, this pending field is set to "Yes".
	// For a100/h100, it requires a GPU reset to actually remap the row.
	// ref. https://docs.nvidia.com/deploy/a100-gpu-mem-error-mgmt/index.html#rma-policy-thresholds
	RemappingPending bool `json:"remapping_pending,omitempty" yaml:"remapping_pending,omitempty"`

	// Yes/No
	RemappingFailureOccurred bool `json:"Remapping Failure Occurred,omitempty" yaml:"Remapping Failure Occurred,omitempty"`
}

type LocationErrors struct {
	// Total ECC error counts for the device.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g9748430b6aa6cdbb2349c5e835d70b0f
	Total ErrorType `json:"TOTAL" yaml:"TOTAL"`

	// Memory locations types.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1g9bcbee49054a953d333d4aa11e8b9c25
	L1Cache          ErrorType `json:"GPU L1 Cache" yaml:"GPU L1 Cache"`
	L2Cache          ErrorType `json:"GPU L2 Cache" yaml:"GPU L2 Cache"`
	DRAM             ErrorType `json:"Turing+DRAM" yaml:"Turing+DRAM"`
	GPUDeviceMemory  ErrorType `json:"GPU Device Memory" yaml:"GPU Device Memory"`
	GPURegisterFile  ErrorType `json:"GPU Register File" yaml:"GPU Register File"`
	GPUTextureMemory ErrorType `json:"GPU Texture Memory" yaml:"GPU Texture Memory"`
	SharedMemory     ErrorType `json:"Shared memory" yaml:"Shared memory"`
	CBU              ErrorType `json:"CBU" yaml:"CBU"`
	SRAM             ErrorType `json:"Turing+SRAM" yaml:"Turing+SRAM"`
	// COUNT            ErrorType `json:"COUNT" yaml:"COUNT"`  ERROR_INVALID_ARGUMENT
}

type ErrorType struct {
	// A memory error that was correctedFor ECC errors, these are single bit errors.
	// For Texture memory, these are errors fixed by resend.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1gc5469bd68b9fdcf78734471d86becb24
	Corrected uint64 `json:"corrected" yaml:"corrected"`

	// A memory error that was not correctedFor ECC errors.
	// these are double bit errors For Texture memory.
	// these are errors where the resend fails.
	// ref. https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceEnumvs.html#group__nvmlDeviceEnumvs_1gc5469bd68b9fdcf78734471d86becb24
	Uncorrected uint64 `json:"uncorrected" yaml:"uncorrected"`
}

func (memErrors *MemoryErrors) JSON() ([]byte, error) {
	return common.JSON(memErrors)
}

// ToString Convert struct to JSON (pretty-printed)
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
	if !errors.Is(ret, nvml.SUCCESS) {
		return fmt.Errorf("failed to get current/pending ecc mode: %s", nvml.ErrorString(ret))
	}

	memErrors.ECCMode.Current = int32(current)
	memErrors.ECCMode.Pending = int32(pending)

	return nil
}

// Get Remapped Rows
func (memErrors *MemoryErrors) getRemappedRows(device nvml.Device, uuid string) error {
	remappedRowsCorrectable, remappedRowsUncorrectable, remappingPending, remappingFailureOccurred, err := device.GetRemappedRows()
	if !errors.Is(err, nvml.SUCCESS) {
		if errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
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
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(Total, Aggregate, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}
	memErrors.AggregateECC.Total.Uncorrected, err = device.GetTotalEccErrors(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(Total, Aggregate, uncorrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.Total.Corrected, err = device.GetTotalEccErrors(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.VOLATILE_ECC,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(Total, Volatile, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.Total.Uncorrected, err = device.GetTotalEccErrors(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
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
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(DRAM, Aggregate, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.DRAM.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_DRAM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(DRAM, Aggregate, Uncorrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.SRAM.Corrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_SRAM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(SRAM, Aggregate, Corrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.SRAM.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_SRAM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(SRAM, Aggregate, UnCorrected) failed to get total ecc errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.SRAM.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_SRAM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(SRAM, Volatile, UnCorrected) get volatile sram uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.SRAM.Corrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_SRAM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(SRAM, Volatile, UnCorrected) get volatile sram corrected errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.L2Cache.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_L2_CACHE,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(L2_CACHE, Volatile, UnCorrected) get volatile l2 cache uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.L1Cache.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_L1_CACHE,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(L1_CACHE, Volatile, UnCorrected) get volatile l1 cache uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.GPURegisterFile.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_REGISTER_FILE,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(REGISTER_FILE, Volatile, UnCorrected) get volatile register file uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.GPUTextureMemory.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_TEXTURE_MEMORY,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(TEXTURE_MEMORY, Volatile, UnCorrected) get volatile texture memory uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.SharedMemory.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_TEXTURE_SHM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(TEXTURE_SHM, Volatile, UnCorrected) get volatile texture shm uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.GPUDeviceMemory.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_DEVICE_MEMORY,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(DEVICE_MEMORY, Volatile, UnCorrected) get volatile device memory uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.CBU.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_CBU,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(CBU, Volatile, UnCorrected) get volatile cbu uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.TotalVolatileECC = memErrors.VolatileECC.L1Cache.Uncorrected +
		memErrors.VolatileECC.L2Cache.Uncorrected +
		memErrors.VolatileECC.GPURegisterFile.Uncorrected +
		memErrors.VolatileECC.GPUTextureMemory.Uncorrected +
		memErrors.VolatileECC.SharedMemory.Uncorrected +
		memErrors.VolatileECC.GPUDeviceMemory.Uncorrected +
		memErrors.VolatileECC.CBU.Uncorrected

	if memErrors.TotalVolatileECC > 0 {
		logrus.WithField("component", "nvidia").Warnf("Detected volatile ecc errors for GPU %s: TotalVolatileECC = %x", uuid, memErrors.TotalVolatileECC)
	}

	memErrors.AggregateECC.L2Cache.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_L2_CACHE,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(L2_CACHE, Aggregate, UnCorrected) get aggregate l2 cache uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.L1Cache.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_L1_CACHE,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(L1_CACHE, Aggregate, UnCorrected) get aggregate l1 cache uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.GPURegisterFile.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_REGISTER_FILE,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(REGISTER_FILE, Aggregate, UnCorrected) get aggregate register file uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.GPUTextureMemory.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_TEXTURE_MEMORY,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(TEXTURE_MEMORY, Aggregate, UnCorrected) get aggregate texture memory uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.SharedMemory.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_TEXTURE_SHM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(TEXTURE_SHM, Aggregate, UnCorrected) get aggregate texture shm uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.GPUDeviceMemory.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_DEVICE_MEMORY,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(DEVICE_MEMORY, Aggregate, UnCorrected) get aggregate device memory uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.AggregateECC.CBU.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.AGGREGATE_ECC,
		nvml.MEMORY_LOCATION_CBU,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(CBU, Aggregate, UnCorrected) get aggregate cbu uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.TotalAggregateECC = memErrors.AggregateECC.L1Cache.Uncorrected +
		memErrors.AggregateECC.L2Cache.Uncorrected +
		memErrors.AggregateECC.GPURegisterFile.Uncorrected +
		memErrors.AggregateECC.GPUTextureMemory.Uncorrected +
		memErrors.AggregateECC.SharedMemory.Uncorrected +
		memErrors.AggregateECC.GPUDeviceMemory.Uncorrected +
		memErrors.AggregateECC.CBU.Uncorrected

	if memErrors.TotalAggregateECC > 0 {
		logrus.WithField("component", "nvidia").Warnf("Detected aggregate ecc errors for GPU %s: TotalAggregateECC = %x", uuid, memErrors.TotalAggregateECC)
	}

	memErrors.VolatileECC.DRAM.Uncorrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_UNCORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_DRAM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(DRAM, Volatile, UnCorrected) get volatile dram uncorrectable errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	memErrors.VolatileECC.DRAM.Corrected, err = device.GetMemoryErrorCounter(
		nvml.MEMORY_ERROR_TYPE_CORRECTED,
		nvml.VOLATILE_ECC,
		nvml.MEMORY_LOCATION_DRAM,
	)
	if !errors.Is(err, nvml.SUCCESS) && !errors.Is(err, nvml.ERROR_NOT_SUPPORTED) {
		result = fmt.Errorf("(DRAM, Volatile, Corrected) get volatile dram corrected errors for GPU %s: %s", uuid, nvml.ErrorString(err))
	}

	return result
}
