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
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const edacSysfsPath = "/sys/devices/system/edac/mc"

// EDACInfo holds aggregated EDAC memory controller error data.
type EDACInfo struct {
	Available   bool     `json:"available"`
	Controllers []MCInfo `json:"controllers"`
	TotalCE     int64    `json:"total_ce"`
	TotalUCE    int64    `json:"total_uce"`
}

// MCInfo represents a single memory controller's error counts.
type MCInfo struct {
	ID       string      `json:"id"`
	CECount  int64       `json:"ce_count"`
	UCECount int64       `json:"uce_count"`
	CSRows   []CSRowInfo `json:"csrows"`
}

// CSRowInfo represents a chip-select row's error counts.
type CSRowInfo struct {
	ID       string `json:"id"`
	CECount  int64  `json:"ce_count"`
	UCECount int64  `json:"uce_count"`
}

// Get reads EDAC info from the default sysfs path.
func (e *EDACInfo) Get() {
	e.getFromDir(filepath.Dir(edacSysfsPath))
}

// getFromDir reads EDAC info from the given base directory (parent of mc/).
func (e *EDACInfo) getFromDir(baseDir string) {
	mcDir := filepath.Join(baseDir, "mc")
	entries, err := os.ReadDir(mcDir)
	if err != nil {
		e.Available = false
		return
	}

	e.Available = true
	e.TotalCE = 0
	e.TotalUCE = 0
	e.Controllers = nil

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "mc") {
			continue
		}

		mcPath := filepath.Join(mcDir, entry.Name())
		mc := MCInfo{
			ID:       entry.Name(),
			CECount:  readEdacIntFile(filepath.Join(mcPath, "ce_count")),
			UCECount: readEdacIntFile(filepath.Join(mcPath, "ue_count")),
		}

		// Read csrow subdirectories
		csEntries, err := os.ReadDir(mcPath)
		if err == nil {
			for _, csEntry := range csEntries {
				if !csEntry.IsDir() || !strings.HasPrefix(csEntry.Name(), "csrow") {
					continue
				}
				csPath := filepath.Join(mcPath, csEntry.Name())
				csRow := CSRowInfo{
					ID:       csEntry.Name(),
					CECount:  readEdacIntFile(filepath.Join(csPath, "ce_count")),
					UCECount: readEdacIntFile(filepath.Join(csPath, "ue_count")),
				}
				mc.CSRows = append(mc.CSRows, csRow)
			}
		}

		e.Controllers = append(e.Controllers, mc)
		e.TotalCE += mc.CECount
		e.TotalUCE += mc.UCECount
	}
}

// readEdacIntFile reads a single integer value from a sysfs file.
// Returns 0 if the file cannot be read or parsed.
func readEdacIntFile(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	if val < 0 {
		return 0
	}
	return val
}
