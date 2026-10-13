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
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	cpuInfoPath     = "/proc/cpuinfo"
	uncoreSysfsPath = "/sys/devices/system/cpu/intel_uncore_frequency"

	UncoreInterfaceTPMI   = "tpmi"
	UncoreInterfaceLegacy = "legacy"
)

// UncoreDomain is one uncore frequency domain (L3/LLC, memory controller, mesh).
// All frequencies are in kHz; 0 means the attribute is not exposed.
type UncoreDomain struct {
	Name              string `json:"name"`
	MinFreqKHz        int64  `json:"min_freq_khz"`
	MaxFreqKHz        int64  `json:"max_freq_khz"`
	InitialMinFreqKHz int64  `json:"initial_min_freq_khz"`
	InitialMaxFreqKHz int64  `json:"initial_max_freq_khz"`
	CurrentFreqKHz    int64  `json:"current_freq_khz"`
}

// Pinned reports whether the band is collapsed onto the hardware ceiling
// (min == max == initial_max), i.e. the PCU can no longer drop uncore frequency.
func (d UncoreDomain) Pinned() bool {
	return d.InitialMaxFreqKHz > 0 && d.MinFreqKHz == d.InitialMaxFreqKHz && d.MaxFreqKHz == d.InitialMaxFreqKHz
}

// UncoreInfo describes the uncore frequency posture read from intel_uncore_frequency sysfs.
type UncoreInfo struct {
	// Applicable is false on non-Intel CPUs and virtual machines, where uncore is not managed by the OS.
	Applicable   bool           `json:"applicable"`
	SkipReason   string         `json:"skip_reason,omitempty"`
	DriverLoaded bool           `json:"driver_loaded"`
	Interface    string         `json:"interface,omitempty"`
	Domains      []UncoreDomain `json:"domains,omitempty"`
}

// Get populates UncoreInfo from the default procfs/sysfs paths. Read-only.
func (u *UncoreInfo) Get() {
	u.getFrom(cpuInfoPath, uncoreSysfsPath)
}

func (u *UncoreInfo) getFrom(cpuinfo, uncoreDir string) {
	*u = UncoreInfo{}
	vendor, hypervisor := readCPUVendorFlags(cpuinfo)
	switch {
	case vendor != "GenuineIntel":
		u.SkipReason = "not an Intel CPU"
		return
	case hypervisor:
		u.SkipReason = "virtual machine"
		return
	}
	u.Applicable = true

	if _, err := os.Stat(uncoreDir); err != nil {
		return
	}
	u.DriverLoaded = true

	// TPMI (uncoreNN, per-domain) and the legacy MSR interface (package_NN_die_NN) can both
	// register and drive the same hardware; judge TPMI when present, legacy otherwise.
	u.Interface = UncoreInterfaceTPMI
	u.Domains = readUncoreDomains(uncoreDir, func(name string) bool { return isTPMIDomain(name) })
	if len(u.Domains) == 0 {
		u.Interface = UncoreInterfaceLegacy
		u.Domains = readUncoreDomains(uncoreDir, func(name string) bool { return !isTPMIDomain(name) })
	}
	if len(u.Domains) == 0 {
		u.Interface = ""
	}
}

// isTPMIDomain matches the TPMI per-domain directories "uncore00", "uncore01", ...
func isTPMIDomain(name string) bool {
	rest := strings.TrimPrefix(name, "uncore")
	if rest == name || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func readUncoreDomains(dir string, match func(string) bool) []UncoreDomain {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var domains []UncoreDomain
	for _, entry := range entries {
		if !entry.IsDir() || !match(entry.Name()) {
			continue
		}
		d := filepath.Join(dir, entry.Name())
		// A domain without a readable min_freq_khz is not a frequency domain (e.g. a bare "uncore" dir).
		if _, err := os.ReadFile(filepath.Join(d, "min_freq_khz")); err != nil {
			continue
		}
		domains = append(domains, UncoreDomain{
			Name:              entry.Name(),
			MinFreqKHz:        readIntFile(filepath.Join(d, "min_freq_khz")),
			MaxFreqKHz:        readIntFile(filepath.Join(d, "max_freq_khz")),
			InitialMinFreqKHz: readIntFile(filepath.Join(d, "initial_min_freq_khz")),
			InitialMaxFreqKHz: readIntFile(filepath.Join(d, "initial_max_freq_khz")),
			CurrentFreqKHz:    readIntFile(filepath.Join(d, "current_freq_khz")),
		})
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Name < domains[j].Name })
	return domains
}

// readCPUVendorFlags returns the vendor_id of the first CPU and whether the
// "hypervisor" flag is set on its flags line.
func readCPUVendorFlags(path string) (vendor string, hypervisor bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	gotFlags := false
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "vendor_id":
			if vendor == "" {
				vendor = strings.TrimSpace(val)
			}
		case "flags":
			if !gotFlags {
				gotFlags = true
				for _, flag := range strings.Fields(val) {
					if flag == "hypervisor" {
						hypervisor = true
					}
				}
			}
		}
		if vendor != "" && gotFlags {
			break
		}
	}
	return vendor, hypervisor
}
