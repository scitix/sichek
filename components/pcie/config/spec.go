package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	nvutils "github.com/scitix/sichek/components/nvidia/utils"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/httpclient" // Added this import
	// Added this import
	"github.com/sirupsen/logrus"
)

// PciDevice represents the device configuration
type PcieTopoSpecs struct {
	Specs map[string]*PcieTopoSpec `json:"pcie_topo" yaml:"pcie_topo"`
}

type PcieTopoSpec struct {
	NumaConfig        []*NumaConfig `json:"numa_config" yaml:"numa_config"`
	PciSwitchesConfig []*PciSwitch  `json:"pci_switches" yaml:"pci_switches"`
}

type NumaConfig struct {
	NodeID   uint64 `json:"node_id" yaml:"node_id"`
	GPUCount int    `json:"gpu_count" yaml:"gpu_count"`
	IBCount  int    `json:"ib_count" yaml:"ib_count"`
}

type PciSwitch struct {
	GPU   int `json:"gpu" yaml:"gpu"`
	IB    int `json:"ib" yaml:"ib"`
	Count int `json:"count" yaml:"count"`
}

type BDFItem struct {
	DeviceType string `json:"type" yaml:"type"`
	BDF        string `json:"bdf" yaml:"bdf"`
}

// ─── EnsureSpec ──────────────────────────────────────────────────────────────
// EnsureSpec ensures that `file` contains a PCIe topology spec entry for the local GPU.
//
// It detects the GPU device ID via NVML. If the entry is already present in
// `file`, it returns immediately. Otherwise it downloads the per-device spec
// from SICHEK_SPEC_URL and merges it into `file` (with backup and tracing).
//
// This should be called after spec.EnsureSpecFile so that `file` already
// contains the cluster-level multi-spec map.
func EnsureSpec(file string) (string, error) {
	const comp = "pcie/spec"

	localDeviceID, err := nvutils.GetDeviceID()
	if err != nil {
		return file, fmt.Errorf("EnsureSpec: cannot detect GPU device ID: %w", err)
	}
	logrus.WithField("component", comp).Infof("local GPU device ID: %s", localDeviceID)

	// Check whether the cluster-level file already has this device
	var s PcieTopoSpecs
	if err := common.LoadSpec(file, &s); err == nil {
		if s.Specs != nil {
			if _, ok := s.Specs[localDeviceID]; ok {
				logrus.WithField("component", comp).Infof("spec for GPU %s already in %s, skipping download", localDeviceID, file)
				return file, nil
			}
		}
	} else {
		logrus.WithField("component", comp).Debugf("LoadSpec failed during EnsureSpec: %v", err)
	}

	// Download {SICHEK_SPEC_URL}/pcie/{deviceID}.yaml
	ossBase := httpclient.GetSichekSpecURL()
	if ossBase == "" {
		return file, fmt.Errorf("EnsureSpec: GPU %s not in pcie_topo spec and SICHEK_SPEC_URL not set", localDeviceID)
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("pcie_%s.yaml", localDeviceID))
	perDevURL := fmt.Sprintf("%s/%s/%s.yaml",
		strings.TrimRight(ossBase, "/"), consts.ComponentNamePCIE, localDeviceID)

	logrus.WithField("component", comp).Infof("downloading per-device spec from %s", perDevURL)
	if err := common.DownloadSpecFile(perDevURL, tmpFile, comp); err != nil {
		return file, fmt.Errorf("EnsureSpec: download failed: %w", err)
	}

	// Load the per-device file and merge into cluster-level file
	var perDevice PcieTopoSpecs
	if err := common.LoadSpec(tmpFile, &perDevice); err != nil {
		return file, fmt.Errorf("EnsureSpec: parse per-device spec: %w", err)
	}

	if err := common.MergeAndWriteSpec(
		file,
		"pcie_topo",
		perDevice.Specs,
		func(c *PcieTopoSpecs) map[string]*PcieTopoSpec { return c.Specs },
		func(c *PcieTopoSpecs, m map[string]*PcieTopoSpec) { c.Specs = m },
	); err != nil {
		return file, fmt.Errorf("EnsureSpec: merge failed: %w", err)
	}

	logrus.WithField("component", comp).Infof("merged GPU %s pcie spec into %s", localDeviceID, file)
	return file, nil
}

// ─── LoadSpec ────────────────────────────────────────────────────────────────

// LoadSpec reads the PCIe topology multi-spec YAML at `file`, detects the
// local GPU device ID, overwrites `file` with only that device's spec
// (the applied baseline), and returns the spec.
// It automatically calls EnsureSpec to guarantee the device entry is present
// (potentially downloading it from OSS if missing).
func LoadSpec(file string) (*PcieTopoSpec, error) {
	if file == "" {
		return nil, fmt.Errorf("pcie spec file path is empty")
	}

	// 1. Ensure the device entry is present (downloads & merges if missing)
	if _, err := EnsureSpec(file); err != nil {
		// Log but proceed; FilterSpec will provide the final definitive error if still missing
		logrus.WithField("component", "pcie/spec").Warnf("EnsureSpec failed: %v", err)
	}

	return FilterSpec(file)
}

// ─── FilterSpec ──────────────────────────────────────────────────────────────

// FilterSpec selects the PCIe topology spec for the local GPU device ID from
// the multi-spec YAML at `file`, overwrites `file` with that single entry
// (the applied baseline), and returns the spec.
//
// The overwrite uses atomic rename with a `.bak` backup and logrus tracing.
// No network calls are made; if deviceID is absent call EnsureSpec first.
func FilterSpec(file string) (*PcieTopoSpec, error) {
	localDeviceID, err := nvutils.GetDeviceID()
	if err != nil {
		return nil, err
	}
	return common.FilterSpec(file, "pcie_topo", localDeviceID,
		func(c *PcieTopoSpecs, id string) (*PcieTopoSpec, bool) {
			spec, ok := c.Specs[id]
			return spec, ok
		},
	)
}
