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

type EthernetSpecConfig struct {
	TargetBond     string `json:"target_bond" yaml:"target_bond"`
	BondMode       string `json:"bond_mode" yaml:"bond_mode"`
	MIIStatus      string `json:"mii_status" yaml:"mii_status"`
	LACPRate       string `json:"lacp_rate" yaml:"lacp_rate"`
	MTU            string `json:"mtu" yaml:"mtu"`
	Speed          string `json:"speed" yaml:"speed"`
	MinSlaves      int    `json:"min_slaves" yaml:"min_slaves"`
	XmitHashPolicy string `json:"xmit_hash_policy" yaml:"xmit_hash_policy"`
	Miimon         int    `json:"miimon" yaml:"miimon"`
	UpDelay        int    `json:"updelay" yaml:"updelay"`
	DownDelay      int    `json:"downdelay" yaml:"downdelay"`
}

type EthernetSpecs struct {
	Specs map[string]*EthernetSpecConfig `json:"ethernet" yaml:"ethernet"`
}

// ─── EnsureSpec ──────────────────────────────────────────────────────────────

// EnsureSpec ensures that `file` contains a spec entry for the "default" ethernet config.
// Since ethernet config is currently not per-device ID but global/default,
// it just ensures the "default" key is present, potentially downloading from OSS.
func EnsureSpec(file string) (string, error) {
	const comp = "ethernet/spec"
	const deviceID = "default"

	// Check whether the cluster-level file already has this device
	var s EthernetSpecs
	if err := common.LoadSpec(file, &s); err == nil {
		if s.Specs != nil {
			if _, ok := s.Specs[deviceID]; ok {
				logrus.WithField("component", comp).Infof("spec for ethernet %s already in %s, skipping download", deviceID, file)
				return file, nil
			}
		}
	} else {
		logrus.WithField("component", comp).Debugf("LoadSpec failed during EnsureSpec (may be new file): %v", err)
	}

	// Download {SICHEK_SPEC_URL}/ethernet/{deviceID}.yaml
	ossBase := httpclient.GetSichekSpecURL()
	if ossBase == "" {
		return file, fmt.Errorf("EnsureSpec: ethernet %s not in spec and SICHEK_SPEC_URL not set", deviceID)
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("ethernet_%s.yaml", deviceID))
	perDevURL := fmt.Sprintf("%s/%s/%s.yaml",
		strings.TrimRight(ossBase, "/"), consts.ComponentNameEthernet, deviceID)

	logrus.WithField("component", comp).Infof("downloading ethernet spec from %s", perDevURL)
	if err := common.DownloadSpecFile(perDevURL, tmpFile, comp); err != nil {
		return file, fmt.Errorf("EnsureSpec: download failed: %w", err)
	}

	// Load the per-device file and merge into cluster-level file
	var perDevice EthernetSpecs
	if err := common.LoadSpec(tmpFile, &perDevice); err != nil {
		return file, fmt.Errorf("EnsureSpec: parse per-device spec: %w", err)
	}

	if err := common.MergeAndWriteSpec(
		file,
		"ethernet",
		perDevice.Specs,
		func(c *EthernetSpecs) map[string]*EthernetSpecConfig { return c.Specs },
		func(c *EthernetSpecs, m map[string]*EthernetSpecConfig) { c.Specs = m },
	); err != nil {
		return file, fmt.Errorf("EnsureSpec: merge failed: %w", err)
	}

	logrus.WithField("component", comp).Infof("merged ethernet %s spec into %s", deviceID, file)
	return file, nil
}

// ─── LoadSpec ────────────────────────────────────────────────────────────────

// LoadSpec reads the Ethernet multi-spec YAML at `file`, ensures the "default" entry is present
// (potentially downloading it from OSS), and overwrites `file` with only that entry.
func LoadSpec(file string) (*EthernetSpecConfig, error) {
	if file == "" {
		return nil, fmt.Errorf("ethernet spec file path is empty")
	}

	// 1. Ensure the "default" entry is present (downloads & merges if missing)
	if _, err := EnsureSpec(file); err != nil {
		logrus.WithField("component", "ethernet/spec").Warnf("EnsureSpec failed: %v", err)
	}

	// 2. Filter for "default"
	return FilterSpec(file, "default")
}

// ─── FilterSpec ──────────────────────────────────────────────────────────────

// FilterSpec selects the entry for `id` from the multi-spec YAML at `file`,
// overwrites `file` with that single entry, and returns the spec.
func FilterSpec(file, id string) (*EthernetSpecConfig, error) {
	logrus.WithField("component", "ethernet").Infof(
		"filtering spec for ethernet %s in %s", id, file)
	return common.FilterSpec(file, "ethernet", id,
		func(c *EthernetSpecs, id string) (*EthernetSpecConfig, bool) {
			spec, ok := c.Specs[id]
			return spec, ok
		},
	)
}
