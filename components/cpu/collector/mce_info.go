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

// edacMCPath is where the kernel EDAC driver (e.g. i10nm_edac, skx_edac,
// amd64_edac) exposes one mcN directory per memory controller, each carrying
// cumulative ce_count / ue_count since boot. The machinecheck sysfs tree has no
// error counters (only bank/config knobs), so EDAC is the counter source.
const edacMCPath = "/sys/devices/system/edac/mc"

// MCEInfo holds memory-controller Machine Check error counters from EDAC.
type MCEInfo struct {
	CorrectedCount   int64 `json:"corrected_count"`
	UncorrectedCount int64 `json:"uncorrected_count"`
	Available        bool  `json:"available"`
}

// Get populates MCEInfo from the default EDAC sysfs path.
func (m *MCEInfo) Get() {
	m.getFromDir(edacMCPath)
}

// getFromDir sums ce_count and ue_count across the mcN directories under dir.
// Available is true only when at least one controller's counters were actually
// read, so a node without a loaded EDAC driver reports "not available" instead
// of a misleading zero.
func (m *MCEInfo) getFromDir(dir string) {
	*m = MCEInfo{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() || !isEDACMCDir(entry.Name()) {
			continue
		}
		mcDir := filepath.Join(dir, entry.Name())

		corrected, err := readCountFile(filepath.Join(mcDir, "ce_count"))
		if err != nil {
			continue
		}
		uncorrected, err := readCountFile(filepath.Join(mcDir, "ue_count"))
		if err != nil {
			continue
		}
		m.CorrectedCount += corrected
		m.UncorrectedCount += uncorrected
		m.Available = true
	}
}

// isEDACMCDir reports whether name is a memory-controller entry (mc0, mc1, ...)
// rather than a sibling such as power or subsystem.
func isEDACMCDir(name string) bool {
	idx, ok := strings.CutPrefix(name, "mc")
	if !ok || idx == "" {
		return false
	}
	_, err := strconv.Atoi(idx)
	return err == nil
}

// readCountFile reads a file and parses its content as an int64, reporting
// read and parse failures so callers can tell "0" from "unreadable".
func readCountFile(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
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
