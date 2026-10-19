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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	intelCPUInfo = "processor\t: 0\nvendor_id\t: GenuineIntel\nflags\t\t: fpu vme hwp hwp_epp\n\nprocessor\t: 1\nvendor_id\t: GenuineIntel\nflags\t\t: fpu vme hwp hwp_epp\n"
	vmCPUInfo    = "processor\t: 0\nvendor_id\t: GenuineIntel\nflags\t\t: fpu vme hypervisor\n"
	amdCPUInfo   = "processor\t: 0\nvendor_id\t: AuthenticAMD\nflags\t\t: fpu vme\n"
)

type fakeDomain struct {
	name                    string
	min, max, imin, imax, c int64
}

func writeCPUInfo(t *testing.T, dir, content string) string {
	p := filepath.Join(dir, "cpuinfo")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func writeUncore(t *testing.T, dir string, domains []fakeDomain) string {
	root := filepath.Join(dir, "intel_uncore_frequency")
	require.NoError(t, os.MkdirAll(root, 0o755))
	for _, d := range domains {
		dd := filepath.Join(root, d.name)
		require.NoError(t, os.MkdirAll(dd, 0o755))
		for f, v := range map[string]int64{
			"min_freq_khz": d.min, "max_freq_khz": d.max,
			"initial_min_freq_khz": d.imin, "initial_max_freq_khz": d.imax, "current_freq_khz": d.c,
		} {
			require.NoError(t, os.WriteFile(filepath.Join(dd, f), []byte(strconv.FormatInt(v, 10)+"\n"), 0o644))
		}
	}
	return root
}

func TestUncoreInfo_GetFrom(t *testing.T) {
	// Mirrors a real Granite Rapids host: both interfaces registered, unpinned.
	gnr := []fakeDomain{
		{"package_00_die_00", 800000, 2500000, 800000, 2500000, 0},
		{"uncore01", 800000, 2200000, 800000, 2200000, 1200000},
		{"uncore00", 800000, 2200000, 800000, 2200000, 1200000},
	}
	tests := []struct {
		name          string
		cpuinfo       string
		domains       []fakeDomain
		noUncoreDir   bool
		wantApplic    bool
		wantDriver    bool
		wantInterface string
		wantDomains   []string
	}{
		{name: "AMD skipped", cpuinfo: amdCPUInfo, domains: gnr, wantApplic: false},
		{name: "VM skipped", cpuinfo: vmCPUInfo, domains: gnr, wantApplic: false},
		{name: "Intel without driver", cpuinfo: intelCPUInfo, noUncoreDir: true, wantApplic: true, wantDriver: false},
		{name: "TPMI preferred over legacy", cpuinfo: intelCPUInfo, domains: gnr,
			wantApplic: true, wantDriver: true, wantInterface: UncoreInterfaceTPMI, wantDomains: []string{"uncore00", "uncore01"}},
		{name: "legacy only", cpuinfo: intelCPUInfo, domains: gnr[:1],
			wantApplic: true, wantDriver: true, wantInterface: UncoreInterfaceLegacy, wantDomains: []string{"package_00_die_00"}},
		{name: "driver dir without domains", cpuinfo: intelCPUInfo, domains: nil,
			wantApplic: true, wantDriver: true, wantInterface: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cpuinfo := writeCPUInfo(t, dir, tt.cpuinfo)
			uncoreDir := filepath.Join(dir, "absent")
			if !tt.noUncoreDir {
				uncoreDir = writeUncore(t, dir, tt.domains)
			}
			var u UncoreInfo
			u.getFrom(cpuinfo, uncoreDir)
			assert.Equal(t, tt.wantApplic, u.Applicable)
			assert.Equal(t, tt.wantDriver, u.DriverLoaded)
			assert.Equal(t, tt.wantInterface, u.Interface)
			var names []string
			for _, d := range u.Domains {
				names = append(names, d.Name)
			}
			assert.Equal(t, tt.wantDomains, names)
			if !tt.wantApplic {
				assert.NotEmpty(t, u.SkipReason)
			}
		})
	}
}

func TestUncoreInfo_ReadsValues(t *testing.T) {
	dir := t.TempDir()
	cpuinfo := writeCPUInfo(t, dir, intelCPUInfo)
	uncoreDir := writeUncore(t, dir, []fakeDomain{{"uncore00", 2200000, 2200000, 800000, 2200000, 2200000}})
	var u UncoreInfo
	u.getFrom(cpuinfo, uncoreDir)
	require.Len(t, u.Domains, 1)
	assert.Equal(t, UncoreDomain{Name: "uncore00", MinFreqKHz: 2200000, MaxFreqKHz: 2200000,
		InitialMinFreqKHz: 800000, InitialMaxFreqKHz: 2200000, CurrentFreqKHz: 2200000}, u.Domains[0])
	assert.True(t, u.Domains[0].Pinned())
}

func TestUncoreDomain_Pinned(t *testing.T) {
	tests := []struct {
		name string
		d    UncoreDomain
		want bool
	}{
		{"collapsed on ceiling", UncoreDomain{MinFreqKHz: 2500000, MaxFreqKHz: 2500000, InitialMaxFreqKHz: 2500000}, true},
		{"default band", UncoreDomain{MinFreqKHz: 800000, MaxFreqKHz: 2500000, InitialMaxFreqKHz: 2500000}, false},
		{"max lowered", UncoreDomain{MinFreqKHz: 2000000, MaxFreqKHz: 2000000, InitialMaxFreqKHz: 2500000}, false},
		{"ceiling unreadable", UncoreDomain{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.d.Pinned())
		})
	}
}
