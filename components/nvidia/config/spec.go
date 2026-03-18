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
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	nvutils "github.com/scitix/sichek/components/nvidia/utils"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

// ─── Spec structs ─────────────────────────────────────────────────────────────

type NvidiaSpec struct {
	Name                 string                 `json:"name" yaml:"name"`
	GpuNums              int                    `json:"gpu_nums" yaml:"gpu_nums"`
	GpuMemory            int                    `json:"gpu_memory" yaml:"gpu_memory"`
	GpuMemoryBandwidth   int                    `json:"gpu_memory_bandwidth,omitempty" yaml:"gpu_memory_bandwidth,omitempty"`
	PCIe                 collector.PCIeInfo     `json:"pcie,omitempty" yaml:"pcie,omitempty"`
	Dependence           Dependence             `json:"dependence" yaml:"dependence"`
	Software             collector.SoftwareInfo `json:"software" yaml:"software"`
	Nvlink               collector.NVLinkStates `json:"nvlink" yaml:"nvlink"`
	State                collector.StatesInfo   `json:"state" yaml:"state"`
	MemoryErrorThreshold MemoryErrorThreshold   `json:"memory_errors_threshold" yaml:"memory_errors_threshold"`
	TemperatureThreshold TemperatureThreshold   `json:"temperature_threshold" yaml:"temperature_threshold"`
	CriticalXidEvents    map[int]string         `json:"critical_xid_events,omitempty" yaml:"critical_xid_events,omitempty"`
	Perf                 PerfMetrics            `json:"perf,omitempty" yaml:"perf,omitempty"`
}

type NvidiaSpecs struct {
	Specs map[string]*NvidiaSpec `json:"nvidia" yaml:"nvidia"`
}

type Dependence struct {
	PcieAcs        string `json:"pcie_acs" yaml:"pcie_acs"`
	Iommu          string `json:"iommu" yaml:"iommu"`
	NvidiaPeermem  string `json:"nv_peermem" yaml:"nv_peermem"`
	FabricManager  string `json:"nv_fabricmanager" yaml:"nv_fabricmanager"`
	CpuPerformance string `json:"cpu_performance" yaml:"cpu_performance"`
}

type MemoryErrorThreshold struct {
	RemappedUncorrectableErrors      uint64 `json:"remapped_uncorrectable_errors" yaml:"remapped_uncorrectable_errors"`
	SRAMVolatileUncorrectableErrors  uint64 `json:"sram_volatile_uncorrectable_errors" yaml:"sram_volatile_uncorrectable_errors"`
	SRAMAggregateUncorrectableErrors uint64 `json:"sram_aggregate_uncorrectable_errors" yaml:"sram_aggregate_uncorrectable_errors"`
	SRAMVolatileCorrectableErrors    uint64 `json:"sram_volatile_correctable_errors" yaml:"sram_volatile_correctable_errors"`
	SRAMAggregateCorrectableErrors   uint64 `json:"sram_aggregate_correctable_errors" yaml:"sram_aggregate_correctable_errors"`
}

type TemperatureThreshold struct {
	Gpu    int `json:"gpu" yaml:"gpu"`
	Memory int `json:"memory" yaml:"memory"`
}

type PerfMetrics struct {
	NcclAllReduceBw float64 `json:"nccl-all-reduce-bw" yaml:"nccl-all-reduce-bw"`
}

// ─── EnsureSpec ──────────────────────────────────────────────────────────────

// EnsureSpec ensures that `file` contains a spec entry for the local GPU.
//
// It detects the GPU device ID via NVML. If the entry is already present in
// `file`, it returns immediately. Otherwise it downloads the per-device spec
// from SICHEK_SPEC_URL and merges it into `file` (with backup and tracing).
//
// This should be called after spec.EnsureSpecFile so that `file` already
// contains the cluster-level multi-GPU map.
func EnsureSpec(file string) (string, error) {
	const comp = "nvidia/spec"

	deviceID, err := nvutils.GetDeviceID()
	if err != nil {
		return file, fmt.Errorf("EnsureSpec: cannot detect GPU device ID: %w", err)
	}
	logrus.WithField("component", comp).Infof("local GPU device ID: %s", deviceID)

	// Check whether the cluster-level file already has this device
	var s NvidiaSpecs
	if err := common.LoadSpec(file, &s); err == nil {
		if _, ok := s.Specs[deviceID]; ok {
			logrus.WithField("component", comp).Infof("spec for GPU %s already in %s", deviceID, file)
			return file, nil
		}
	}

	// Download {SICHEK_SPEC_URL}/nvidia/{deviceID}.yaml
	ossBase := os.Getenv("SICHEK_SPEC_URL")
	if ossBase == "" {
		return file, fmt.Errorf("EnsureSpec: GPU %s not in spec and SICHEK_SPEC_URL not set", deviceID)
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("%s.yaml", deviceID))
	perDevURL := fmt.Sprintf("%s/%s/%s.yaml",
		strings.TrimRight(ossBase, "/"), consts.ComponentNameNvidia, deviceID)

	logrus.WithField("component", comp).Infof("downloading per-device spec from %s", perDevURL)
	if err := common.DownloadSpecFile(perDevURL, tmpFile, comp); err != nil {
		return file, fmt.Errorf("EnsureSpec: download failed: %w", err)
	}

	// Load the per-device file and merge into cluster-level file
	var perDevice NvidiaSpecs
	if err := common.LoadSpec(tmpFile, &perDevice); err != nil {
		return file, fmt.Errorf("EnsureSpec: parse per-device spec: %w", err)
	}

	if err := common.MergeAndWriteSpec(
		file,
		"nvidia",
		perDevice.Specs,
		func(c *NvidiaSpecs) map[string]*NvidiaSpec { return c.Specs },
		func(c *NvidiaSpecs, m map[string]*NvidiaSpec) { c.Specs = m },
	); err != nil {
		return file, fmt.Errorf("EnsureSpec: merge failed: %w", err)
	}

	logrus.WithField("component", comp).Infof("merged GPU %s spec into %s", deviceID, file)
	return file, nil
}

// ─── LoadSpec ────────────────────────────────────────────────────────────────

// LoadSpec reads the NVIDIA multi-spec YAML at `file`, detects the local GPU,
// and overwrites `file` with only that GPU's spec (the applied baseline).
// It automatically calls EnsureSpec to guarantee the GPU entry is present
// (potentially downloading it from OSS if missing).
func LoadSpec(file string) (*NvidiaSpec, error) {
	if file == "" {
		return nil, fmt.Errorf("nvidia spec file path is empty")
	}

	// 1. Ensure the device entry is present (downloads & merges if missing)
	if _, err := EnsureSpec(file); err != nil {
		// Log but proceed; FilterSpec will provide the final definitive error if still missing
		logrus.WithField("component", "nvidia/spec").Warnf("EnsureSpec failed: %v", err)
	}

	// 2. Detect and filter
	deviceID, err := nvutils.GetDeviceID()
	if err != nil {
		return nil, fmt.Errorf("LoadSpec: cannot detect GPU device ID: %w", err)
	}
	return FilterSpec(file, deviceID)
}

// ─── FilterSpec ──────────────────────────────────────────────────────────────

// FilterSpec selects the entry for `deviceID` from the multi-spec YAML at
// `file`, overwrites `file` with that single entry (the applied baseline),
// and returns the spec.
//
// The overwrite uses atomic rename with a `.bak` backup and logrus tracing.
// No network calls are made; if deviceID is absent call EnsureSpec first.
func FilterSpec(file, deviceID string) (*NvidiaSpec, error) {
	logrus.WithField("component", "nvidia").Infof(
		"filtering spec for GPU %s in %s", deviceID, file)
	return common.FilterSpec(file, "nvidia", deviceID,
		func(c *NvidiaSpecs, id string) (*NvidiaSpec, bool) {
			spec, ok := c.Specs[id]
			return spec, ok
		},
	)
}
