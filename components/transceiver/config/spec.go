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
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/httpclient"
	"github.com/sirupsen/logrus"
)

// ─── Spec structs ─────────────────────────────────────────────────────────────

type TransceiverSpec struct {
	Networks map[string]*NetworkSpec `json:"networks" yaml:"networks"`
}

type TransceiverSpecs struct {
	Specs map[string]*TransceiverSpec `json:"transceiver" yaml:"transceiver"`
}

type NetworkSpec struct {
	InterfacePatterns []string      `json:"interface_patterns" yaml:"interface_patterns"`
	MaxSpeedMbps      int           `json:"max_speed_mbps" yaml:"max_speed_mbps"`
	Thresholds        ThresholdSpec `json:"thresholds" yaml:"thresholds"`
	CheckVendor       bool          `json:"check_vendor" yaml:"check_vendor"`
	CheckLinkErrors   bool          `json:"check_link_errors" yaml:"check_link_errors"`
	ApprovedVendors   []string      `json:"approved_vendors" yaml:"approved_vendors"`
}

type ThresholdSpec struct {
	TxPowerMarginDB      float64 `json:"tx_power_margin_db" yaml:"tx_power_margin_db"`
	RxPowerMarginDB      float64 `json:"rx_power_margin_db" yaml:"rx_power_margin_db"`
	TemperatureWarningC  float64 `json:"temperature_warning_c" yaml:"temperature_warning_c"`
	TemperatureCriticalC float64 `json:"temperature_critical_c" yaml:"temperature_critical_c"`
}

// ─── EnsureSpec ──────────────────────────────────────────────────────────────

// EnsureSpec ensures that `file` contains a spec entry for the "default" transceiver config.
// If the entry is already present, it returns immediately. Otherwise it downloads
// from SICHEK_SPEC_URL and merges it into `file`.
func EnsureSpec(file string) (string, error) {
	const comp = "transceiver/spec"
	const deviceID = "default"

	// Check whether the cluster-level file already has this device
	var s TransceiverSpecs
	if err := common.LoadSpec(file, &s); err == nil {
		if s.Specs != nil {
			if _, ok := s.Specs[deviceID]; ok {
				logrus.WithField("component", comp).Infof("spec for transceiver %s already in %s, skipping download", deviceID, file)
				return file, nil
			}
		}
	} else {
		logrus.WithField("component", comp).Debugf("LoadSpec failed during EnsureSpec (may be new file): %v", err)
	}

	// Download {SICHEK_SPEC_URL}/transceiver/{deviceID}.yaml
	ossBase := httpclient.GetSichekSpecURL()
	if ossBase == "" {
		return file, fmt.Errorf("EnsureSpec: transceiver %s not in spec and SICHEK_SPEC_URL not set", deviceID)
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("transceiver_%s.yaml", deviceID))
	perDevURL := fmt.Sprintf("%s/%s/%s.yaml",
		strings.TrimRight(ossBase, "/"), consts.ComponentNameTransceiver, deviceID)

	logrus.WithField("component", comp).Infof("downloading transceiver spec from %s", perDevURL)
	if err := common.DownloadSpecFile(perDevURL, tmpFile, comp); err != nil {
		return file, fmt.Errorf("EnsureSpec: download failed: %w", err)
	}

	// Load the per-device file and merge into cluster-level file
	var perDevice TransceiverSpecs
	if err := common.LoadSpec(tmpFile, &perDevice); err != nil {
		return file, fmt.Errorf("EnsureSpec: parse per-device spec: %w", err)
	}

	if err := common.MergeAndWriteSpec(
		file,
		"transceiver",
		perDevice.Specs,
		func(c *TransceiverSpecs) map[string]*TransceiverSpec { return c.Specs },
		func(c *TransceiverSpecs, m map[string]*TransceiverSpec) { c.Specs = m },
	); err != nil {
		return file, fmt.Errorf("EnsureSpec: merge failed: %w", err)
	}

	logrus.WithField("component", comp).Infof("merged transceiver %s spec into %s", deviceID, file)
	return file, nil
}

// ─── LoadSpec ────────────────────────────────────────────────────────────────

// LoadSpec reads the transceiver multi-spec YAML at `file`, ensures the "default" entry
// is present (potentially downloading from OSS), and returns the spec.
func LoadSpec(file string) (*TransceiverSpec, error) {
	if file == "" {
		return loadFromDevDefault()
	}

	// 1. Ensure the "default" entry is present (downloads & merges if missing)
	if _, err := EnsureSpec(file); err != nil {
		logrus.WithField("component", "transceiver/spec").Warnf("EnsureSpec failed: %v", err)
	}

	// 2. Filter for "default"
	spec, err := FilterSpec(file, "default")
	if err != nil {
		logrus.WithField("component", "transceiver/spec").Warnf("FilterSpec failed: %v, using defaults", err)
		return loadFromDevDefault()
	}
	return spec, nil
}

// ─── FilterSpec ──────────────────────────────────────────────────────────────

// FilterSpec selects the entry for `id` from the multi-spec YAML at `file`,
// overwrites `file` with that single entry, and returns the spec.
func FilterSpec(file, id string) (*TransceiverSpec, error) {
	logrus.WithField("component", "transceiver").Infof(
		"filtering spec for transceiver %s in %s", id, file)
	return common.FilterSpec(file, "transceiver", id,
		func(c *TransceiverSpecs, id string) (*TransceiverSpec, bool) {
			spec, ok := c.Specs[id]
			return spec, ok
		},
	)
}

// ─── Dev default fallback ────────────────────────────────────────────────────

func loadFromDevDefault() (*TransceiverSpec, error) {
	cfgDir, files, err := common.GetDevDefaultConfigFiles("transceiver")
	if err != nil {
		logrus.WithField("component", "transceiver").Warnf("failed to get default config dir: %v, using built-in defaults", err)
		return defaultSpec(), nil
	}
	for _, f := range files {
		if f.Name() == "default_spec.yaml" {
			spec := &TransceiverSpec{}
			if err := common.LoadSpec(cfgDir+"/"+f.Name(), spec); err == nil && spec.Networks != nil {
				return spec, nil
			}
		}
	}
	return defaultSpec(), nil
}

func defaultSpec() *TransceiverSpec {
	return &TransceiverSpec{
		Networks: map[string]*NetworkSpec{
			"management": {
				MaxSpeedMbps: 100000,
				Thresholds: ThresholdSpec{
					TxPowerMarginDB: 3.0, RxPowerMarginDB: 3.0,
					TemperatureWarningC: 75, TemperatureCriticalC: 85,
				},
				CheckVendor: false, CheckLinkErrors: false,
			},
			"business": {
				Thresholds: ThresholdSpec{
					TxPowerMarginDB: 1.0, RxPowerMarginDB: 1.0,
					TemperatureWarningC: 65, TemperatureCriticalC: 75,
				},
				CheckVendor: true, CheckLinkErrors: true,
				ApprovedVendors: []string{"Mellanox", "NVIDIA", "Innolight", "Hisense"},
			},
		},
	}
}
