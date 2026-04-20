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

const mceSysfsPath = "/sys/devices/system/cpu/machinecheck"

// MCEInfo holds Machine Check Exception counters.
type MCEInfo struct {
	CorrectedCount   int64 `json:"corrected_count"`
	UncorrectedCount int64 `json:"uncorrected_count"`
	Available        bool  `json:"available"`
}

// Get populates MCEInfo from the default sysfs path.
func (m *MCEInfo) Get() {
	m.getFromDir(mceSysfsPath)
}

// getFromDir reads machinecheck directories under the given path
// and sums corrected_count and uncorrected_count files.
func (m *MCEInfo) getFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.Available = false
		return
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "machinecheck") {
			continue
		}
		found = true
		mcDir := filepath.Join(dir, entry.Name())

		corrected := readIntFile(filepath.Join(mcDir, "corrected_count"))
		m.CorrectedCount += corrected

		uncorrected := readIntFile(filepath.Join(mcDir, "uncorrected_count"))
		m.UncorrectedCount += uncorrected
	}

	m.Available = found
}

// readIntFile reads a file and parses its content as an int64.
// Returns 0 if the file cannot be read or parsed.
func readIntFile(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return val
}
