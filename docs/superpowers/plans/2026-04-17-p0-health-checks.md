# P0 Health Checks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the CPU component with clock sync (PTP/NTP) and MCE checkers, and the Memory component with ECC/EDAC and capacity checkers.

**Architecture:** Add new collector files (ptp_info.go, mce_info.go, edac_info.go, memory_capacity.go) to gather data from sysfs/procfs/systemctl. Add new checker files that consume the collector output and produce CheckerResults. Extend existing CPUOutput and memory Output structs. Add event rules for log-based detection. No new components — all changes extend existing cpu and memory components.

**Tech Stack:** Go 1.23, sysfs/procfs, systemctl, existing EventFilter framework

---

### Task 1: Add constants for new checkers

**Files:**
- Modify: `consts/consts.go`

- [ ] **Step 1: Add checker ID constants**

Add these constants after the existing checker ID block (after line ~110 in consts.go):

```go
// CPU extended checker IDs
CheckerIDClockSyncService  = "1300"
CheckerIDClockSyncOffset   = "1301"
CheckerIDCPUMCEUncorrected = "1302"
CheckerIDCPUMCECorrected   = "1303"

// Memory extended checker IDs
CheckerIDMemoryECCUncorrected = "2100"
CheckerIDMemoryECCCorrected   = "2101"
CheckerIDMemoryCapacity       = "2102"
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./consts/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add consts/consts.go
git commit -m "feat(consts): add checker IDs for clock sync, MCE, and memory ECC"
```

---

### Task 2: CPU collector — PTP/NTP info

**Files:**
- Create: `components/cpu/collector/ptp_info.go`
- Create: `components/cpu/collector/ptp_info_test.go`

- [ ] **Step 1: Write test for PTPInfo.Get()**

Create `components/cpu/collector/ptp_info_test.go`:

```go
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPTPInfoGet(t *testing.T) {
	info := &PTPInfo{}
	// Get() should not error even if PTP/NTP services are not installed
	err := info.Get()
	assert.NoError(t, err)
	// At minimum, the struct should be populated (services may be inactive)
	// We can't assert specific values since this depends on the host
	t.Logf("PTPInfo: PTPActive=%v, PHC2SysActive=%v, NTPActive=%v, OffsetNs=%.2f",
		info.PTPServiceActive, info.PHC2SysActive, info.NTPServiceActive, info.OffsetNs)
}

func TestParseChronycOffset(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantNs  float64
		wantOk  bool
	}{
		{
			name:   "valid chronyc output",
			output: "Reference ID    : C0A80001 (192.168.0.1)\nStratum         : 3\nRef time (UTC)  : Thu Apr 17 10:00:00 2026\nSystem time     : 0.000012345 seconds fast of NTP time\nLast offset     : +0.000005678 seconds\nRMS offset      : 0.000010000 seconds\nFrequency       : 1.234 ppm slow\nResidual freq   : +0.001 ppm\nSkew            : 0.050 ppm\nRoot delay      : 0.012345678 seconds\nRoot dispersion : 0.001234567 seconds\nUpdate interval : 64.0 seconds\nLeap status     : Normal",
			wantNs: 5678.0,
			wantOk: true,
		},
		{
			name:   "negative offset",
			output: "Last offset     : -0.001234000 seconds\n",
			wantNs: -1234000.0,
			wantOk: true,
		},
		{
			name:   "no offset line",
			output: "some random output\n",
			wantNs: 0,
			wantOk: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNs, gotOk := parseChronycOffset(tt.output)
			assert.Equal(t, tt.wantOk, gotOk)
			if tt.wantOk {
				assert.InDelta(t, tt.wantNs, gotNs, 1.0)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/cpu/collector/ -run TestPTPInfoGet -v`
Expected: FAIL — `PTPInfo` undefined

- [ ] **Step 3: Implement ptp_info.go**

Create `components/cpu/collector/ptp_info.go`:

```go
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
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// PTPInfo holds PTP and NTP clock synchronization status.
type PTPInfo struct {
	PTPServiceActive bool    `json:"ptp_service_active"`
	PHC2SysActive    bool    `json:"phc2sys_active"`
	OffsetNs         float64 `json:"offset_ns"`
	NTPServiceActive bool    `json:"ntp_service_active"`
	NTPOffset        float64 `json:"ntp_offset_ns"`
	SyncAvailable    bool    `json:"sync_available"`
}

// Get populates PTPInfo by checking system services.
func (p *PTPInfo) Get() error {
	p.PTPServiceActive = isServiceActive("ptp4l")
	p.PHC2SysActive = isServiceActive("phc2sys")

	if p.PTPServiceActive {
		if offset, ok := getPTP4LOffset(); ok {
			p.OffsetNs = offset
			p.SyncAvailable = true
			return nil
		}
	}

	// Fallback to NTP (chrony first, then ntpd)
	p.NTPServiceActive = isServiceActive("chronyd") || isServiceActive("chrony") || isServiceActive("ntpd")
	if p.NTPServiceActive {
		if offset, ok := getChronycOffset(); ok {
			p.NTPOffset = offset
			p.OffsetNs = offset
			p.SyncAvailable = true
			return nil
		}
	}

	p.SyncAvailable = p.PTPServiceActive || p.NTPServiceActive
	return nil
}

// OffsetMs returns the absolute offset in milliseconds.
func (p *PTPInfo) OffsetMs() float64 {
	return math.Abs(p.OffsetNs) / 1e6
}

func isServiceActive(name string) bool {
	cmd := exec.Command("systemctl", "is-active", "--quiet", name)
	err := cmd.Run()
	return err == nil
}

func getPTP4LOffset() (float64, bool) {
	// Try journalctl for ptp4l offset
	cmd := exec.Command("journalctl", "-u", "ptp4l", "-n", "10", "--no-pager", "-q")
	out, err := cmd.Output()
	if err != nil {
		logrus.WithField("collector", "ptp_info").Debugf("failed to read ptp4l journal: %v", err)
		return 0, false
	}
	return parsePTP4LOffset(string(out))
}

// parsePTP4LOffset extracts the latest offset from ptp4l log output.
// ptp4l log format: "ptp4l[...]: master offset   -5 s2 freq  +1234 path delay  567"
func parsePTP4LOffset(output string) (float64, bool) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		idx := strings.Index(line, "master offset")
		if idx < 0 {
			continue
		}
		parts := strings.Fields(line[idx:])
		if len(parts) >= 3 {
			offset, err := strconv.ParseFloat(parts[2], 64)
			if err == nil {
				return offset, true
			}
		}
	}
	return 0, false
}

func getChronycOffset() (float64, bool) {
	cmd := exec.Command("chronyc", "tracking")
	out, err := cmd.Output()
	if err != nil {
		logrus.WithField("collector", "ptp_info").Debugf("failed to run chronyc tracking: %v", err)
		return 0, false
	}
	return parseChronycOffset(string(out))
}

// parseChronycOffset extracts offset from chronyc tracking output.
// Line format: "Last offset     : +0.000005678 seconds"
func parseChronycOffset(output string) (float64, bool) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "Last offset") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) < 1 {
			continue
		}
		seconds, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		return seconds * 1e9, true // Convert seconds to nanoseconds
	}
	return 0, false
}

func init() {
	// Verify format strings compile
	_ = fmt.Sprintf
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/cpu/collector/ -run "TestPTPInfo|TestParseChronyc" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/cpu/collector/ptp_info.go components/cpu/collector/ptp_info_test.go
git commit -m "feat(cpu): add PTP/NTP clock sync collector"
```

---

### Task 3: CPU collector — MCE info

**Files:**
- Create: `components/cpu/collector/mce_info.go`
- Create: `components/cpu/collector/mce_info_test.go`

- [ ] **Step 1: Write tests**

Create `components/cpu/collector/mce_info_test.go`:

```go
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCEInfoGetFromSysfs(t *testing.T) {
	// Create a fake sysfs tree
	tmpDir := t.TempDir()
	mc0 := filepath.Join(tmpDir, "machinecheck0")
	require.NoError(t, os.MkdirAll(mc0, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mc0, "corrected_count"), []byte("5\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(mc0, "uncorrected_count"), []byte("0\n"), 0644))

	mc1 := filepath.Join(tmpDir, "machinecheck1")
	require.NoError(t, os.MkdirAll(mc1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mc1, "corrected_count"), []byte("3\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(mc1, "uncorrected_count"), []byte("1\n"), 0644))

	info := &MCEInfo{}
	err := info.getFromDir(tmpDir)
	assert.NoError(t, err)
	assert.True(t, info.Available)
	assert.Equal(t, int64(8), info.CorrectedCount)
	assert.Equal(t, int64(1), info.UncorrectedCount)
}

func TestMCEInfoGetFromSysfs_NotAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	// Empty dir, no machinecheck* subdirectories
	info := &MCEInfo{}
	err := info.getFromDir(tmpDir)
	assert.NoError(t, err)
	assert.False(t, info.Available)
	assert.Equal(t, int64(0), info.CorrectedCount)
	assert.Equal(t, int64(0), info.UncorrectedCount)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/cpu/collector/ -run TestMCEInfo -v`
Expected: FAIL — `MCEInfo` undefined

- [ ] **Step 3: Implement mce_info.go**

Create `components/cpu/collector/mce_info.go`:

```go
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

	"github.com/sirupsen/logrus"
)

const mceSysfsPath = "/sys/devices/system/cpu/machinecheck"

// MCEInfo holds Machine Check Exception counters.
type MCEInfo struct {
	CorrectedCount   int64 `json:"corrected_count"`
	UncorrectedCount int64 `json:"uncorrected_count"`
	Available        bool  `json:"available"`
}

// Get reads MCE counters from sysfs.
func (m *MCEInfo) Get() error {
	return m.getFromDir(mceSysfsPath)
}

// getFromDir reads MCE counters from a given directory (testable).
func (m *MCEInfo) getFromDir(baseDir string) error {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		// Directory doesn't exist — MCE monitoring not available
		logrus.WithField("collector", "mce_info").Debugf("MCE sysfs not available at %s: %v", baseDir, err)
		m.Available = false
		return nil
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "machinecheck") {
			continue
		}
		found = true
		mcDir := filepath.Join(baseDir, entry.Name())

		ce := readIntFile(filepath.Join(mcDir, "corrected_count"))
		uce := readIntFile(filepath.Join(mcDir, "uncorrected_count"))
		m.CorrectedCount += ce
		m.UncorrectedCount += uce
	}

	m.Available = found
	return nil
}

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
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/cpu/collector/ -run TestMCEInfo -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/cpu/collector/mce_info.go components/cpu/collector/mce_info_test.go
git commit -m "feat(cpu): add MCE counter collector"
```

---

### Task 4: Extend CPUOutput and CPU collector to include PTP + MCE

**Files:**
- Modify: `components/cpu/collector/collector.go`

- [ ] **Step 1: Add PTPInfo and MCEInfo fields to CPUOutput**

In `components/cpu/collector/collector.go`, add the new fields to the `CPUOutput` struct:

```go
type CPUOutput struct {
	Time        time.Time   `json:"time"`
	CPUArchInfo CPUArchInfo `json:"cpu_arch_info"`
	UsageInfo   Usage       `json:"cpu_usage_info"`
	HostInfo    HostInfo    `json:"host_info"`
	Uptime      string      `json:"uptime"`
	PTPInfo     PTPInfo     `json:"ptp_info"`
	MCEInfo     MCEInfo     `json:"mce_info"`
}
```

- [ ] **Step 2: Collect PTP and MCE data in Collect()**

In the `Collect` method, add PTP and MCE collection after the existing fields. Add these lines before the `return cpuOutput, nil`:

```go
	if err := cpuOutput.PTPInfo.Get(); err != nil {
		logrus.WithField("collector", "cpu").Warnf("failed to collect PTP info: %v", err)
	}
	if err := cpuOutput.MCEInfo.Get(); err != nil {
		logrus.WithField("collector", "cpu").Warnf("failed to collect MCE info: %v", err)
	}
```

- [ ] **Step 3: Verify it compiles and existing tests pass**

Run: `go build ./components/cpu/... && go test ./components/cpu/... -v`
Expected: compiles, existing tests pass

- [ ] **Step 4: Commit**

```bash
git add components/cpu/collector/collector.go
git commit -m "feat(cpu): extend CPUOutput with PTP and MCE info"
```

---

### Task 5: CPU check items — add templates for new checkers

**Files:**
- Modify: `components/cpu/config/check_items.go`

- [ ] **Step 1: Add new checker result templates**

Add these entries to the `CPUCheckItems` map in `components/cpu/config/check_items.go`:

```go
"clock-sync-service": {
	Name:        "clock-sync-service",
	Description: "Check if PTP or NTP clock sync service is running",
	Spec:        "Running",
	Status:      "",
	Level:       consts.LevelWarning,
	Detail:      "",
	ErrorName:   "ClockSyncServiceNotRunning",
	Suggestion:  "Start ptp4l/phc2sys for PTP sync, or chrony/ntpd for NTP sync",
},
"clock-sync-offset": {
	Name:        "clock-sync-offset",
	Description: "Check clock sync offset is within threshold",
	Spec:        "<1ms",
	Status:      "",
	Level:       consts.LevelWarning,
	Detail:      "",
	ErrorName:   "ClockSyncOffsetHigh",
	Suggestion:  "Check PTP/NTP configuration and network connectivity to time source",
},
"cpu-mce-uncorrected": {
	Name:        "cpu-mce-uncorrected",
	Description: "Check for uncorrectable Machine Check Exceptions",
	Spec:        "0",
	Status:      "",
	Level:       consts.LevelCritical,
	Detail:      "",
	ErrorName:   "CPUMCEUncorrected",
	Suggestion:  "CPU hardware error detected. Schedule maintenance and migrate workloads",
},
"cpu-mce-corrected": {
	Name:        "cpu-mce-corrected",
	Description: "Check correctable MCE count is below threshold",
	Spec:        "<10",
	Status:      "",
	Level:       consts.LevelWarning,
	Detail:      "",
	ErrorName:   "CPUMCECorrectedHigh",
	Suggestion:  "Correctable CPU errors increasing. Monitor trend and plan maintenance",
},
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./components/cpu/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add components/cpu/config/check_items.go
git commit -m "feat(cpu): add check item templates for clock sync and MCE checkers"
```

---

### Task 6: CPU checker — clock sync

**Files:**
- Create: `components/cpu/checker/clock_sync.go`
- Create: `components/cpu/checker/clock_sync_test.go`

- [ ] **Step 1: Write tests**

Create `components/cpu/checker/clock_sync_test.go`:

```go
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
package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClockSyncServiceChecker(t *testing.T) {
	tests := []struct {
		name       string
		ptpInfo    collector.PTPInfo
		wantStatus string
	}{
		{
			name:       "PTP active",
			ptpInfo:    collector.PTPInfo{PTPServiceActive: true, SyncAvailable: true},
			wantStatus: consts.StatusNormal,
		},
		{
			name:       "NTP active, PTP inactive",
			ptpInfo:    collector.PTPInfo{NTPServiceActive: true, SyncAvailable: true},
			wantStatus: consts.StatusNormal,
		},
		{
			name:       "no sync service",
			ptpInfo:    collector.PTPInfo{SyncAvailable: false},
			wantStatus: consts.StatusAbnormal,
		},
	}

	checker, err := NewClockSyncServiceChecker()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &collector.CPUOutput{PTPInfo: tt.ptpInfo}
			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}

func TestClockSyncOffsetChecker(t *testing.T) {
	tests := []struct {
		name       string
		ptpInfo    collector.PTPInfo
		wantStatus string
		wantLevel  string
	}{
		{
			name:       "offset within threshold",
			ptpInfo:    collector.PTPInfo{SyncAvailable: true, OffsetNs: 500000}, // 0.5ms
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelWarning,
		},
		{
			name:       "offset exceeds warning threshold",
			ptpInfo:    collector.PTPInfo{SyncAvailable: true, OffsetNs: 2000000}, // 2ms
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelWarning,
		},
		{
			name:       "offset exceeds critical threshold",
			ptpInfo:    collector.PTPInfo{SyncAvailable: true, OffsetNs: 15000000}, // 15ms
			wantStatus: consts.StatusAbnormal,
			wantLevel:  consts.LevelCritical,
		},
		{
			name:       "no sync available — skip",
			ptpInfo:    collector.PTPInfo{SyncAvailable: false},
			wantStatus: consts.StatusNormal,
			wantLevel:  consts.LevelWarning,
		},
	}

	checker, err := NewClockSyncOffsetChecker(1.0, 10.0)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &collector.CPUOutput{PTPInfo: tt.ptpInfo}
			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantLevel, result.Level)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/cpu/checker/ -run TestClockSync -v`
Expected: FAIL — `NewClockSyncServiceChecker` undefined

- [ ] **Step 3: Implement clock_sync.go**

Create `components/cpu/checker/clock_sync.go`:

```go
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
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/consts"
)

const (
	ClockSyncServiceCheckerName = "clock-sync-service"
	ClockSyncOffsetCheckerName  = "clock-sync-offset"
)

// ClockSyncServiceChecker checks if PTP or NTP service is running.
type ClockSyncServiceChecker struct {
	name string
}

func NewClockSyncServiceChecker() (common.Checker, error) {
	return &ClockSyncServiceChecker{name: ClockSyncServiceCheckerName}, nil
}

func (c *ClockSyncServiceChecker) Name() string { return c.name }

func (c *ClockSyncServiceChecker) GetSpec() common.CheckerSpec { return nil }

func (c *ClockSyncServiceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuInfo, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type for ClockSyncServiceChecker")
	}

	result := config.CPUCheckItems[ClockSyncServiceCheckerName]

	if cpuInfo.PTPInfo.SyncAvailable {
		result.Status = consts.StatusNormal
		if cpuInfo.PTPInfo.PTPServiceActive {
			result.Curr = "PTP active"
		} else {
			result.Curr = "NTP active"
		}
	} else {
		result.Status = consts.StatusAbnormal
		result.Curr = "No sync service"
		result.Detail = "Neither PTP (ptp4l) nor NTP (chrony/ntpd) service is running"
	}

	return &result, nil
}

// ClockSyncOffsetChecker checks if clock offset is within thresholds.
type ClockSyncOffsetChecker struct {
	name             string
	offsetWarningMs  float64
	offsetCriticalMs float64
}

func NewClockSyncOffsetChecker(warningMs, criticalMs float64) (common.Checker, error) {
	return &ClockSyncOffsetChecker{
		name:             ClockSyncOffsetCheckerName,
		offsetWarningMs:  warningMs,
		offsetCriticalMs: criticalMs,
	}, nil
}

func (c *ClockSyncOffsetChecker) Name() string { return c.name }

func (c *ClockSyncOffsetChecker) GetSpec() common.CheckerSpec { return nil }

func (c *ClockSyncOffsetChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuInfo, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type for ClockSyncOffsetChecker")
	}

	result := config.CPUCheckItems[ClockSyncOffsetCheckerName]

	if !cpuInfo.PTPInfo.SyncAvailable {
		result.Status = consts.StatusNormal
		result.Curr = "N/A (no sync service)"
		return &result, nil
	}

	offsetMs := cpuInfo.PTPInfo.OffsetMs()
	result.Curr = fmt.Sprintf("%.3fms", offsetMs)

	switch {
	case offsetMs >= c.offsetCriticalMs:
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.Detail = fmt.Sprintf("Clock offset %.3fms exceeds critical threshold %.1fms", offsetMs, c.offsetCriticalMs)
	case offsetMs >= c.offsetWarningMs:
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Clock offset %.3fms exceeds warning threshold %.1fms", offsetMs, c.offsetWarningMs)
	default:
		result.Status = consts.StatusNormal
	}

	return &result, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/cpu/checker/ -run TestClockSync -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/cpu/checker/clock_sync.go components/cpu/checker/clock_sync_test.go
git commit -m "feat(cpu): add clock sync service and offset checkers"
```

---

### Task 7: CPU checker — MCE

**Files:**
- Create: `components/cpu/checker/cpu_mce.go`
- Create: `components/cpu/checker/cpu_mce_test.go`

- [ ] **Step 1: Write tests**

Create `components/cpu/checker/cpu_mce_test.go`:

```go
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
package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCPUMCEUncorrectedChecker(t *testing.T) {
	tests := []struct {
		name       string
		mceInfo    collector.MCEInfo
		wantStatus string
	}{
		{
			name:       "no UCE",
			mceInfo:    collector.MCEInfo{Available: true, UncorrectedCount: 0},
			wantStatus: consts.StatusNormal,
		},
		{
			name:       "UCE detected",
			mceInfo:    collector.MCEInfo{Available: true, UncorrectedCount: 1},
			wantStatus: consts.StatusAbnormal,
		},
		{
			name:       "MCE not available",
			mceInfo:    collector.MCEInfo{Available: false},
			wantStatus: consts.StatusNormal,
		},
	}

	checker, err := NewCPUMCEUncorrectedChecker()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &collector.CPUOutput{MCEInfo: tt.mceInfo}
			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}

func TestCPUMCECorrectedChecker(t *testing.T) {
	tests := []struct {
		name       string
		mceInfo    collector.MCEInfo
		threshold  int64
		wantStatus string
	}{
		{
			name:       "CE below threshold",
			mceInfo:    collector.MCEInfo{Available: true, CorrectedCount: 5},
			threshold:  10,
			wantStatus: consts.StatusNormal,
		},
		{
			name:       "CE above threshold",
			mceInfo:    collector.MCEInfo{Available: true, CorrectedCount: 15},
			threshold:  10,
			wantStatus: consts.StatusAbnormal,
		},
		{
			name:       "MCE not available",
			mceInfo:    collector.MCEInfo{Available: false},
			threshold:  10,
			wantStatus: consts.StatusNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewCPUMCECorrectedChecker(tt.threshold)
			require.NoError(t, err)
			data := &collector.CPUOutput{MCEInfo: tt.mceInfo}
			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/cpu/checker/ -run TestCPUMCE -v`
Expected: FAIL — `NewCPUMCEUncorrectedChecker` undefined

- [ ] **Step 3: Implement cpu_mce.go**

Create `components/cpu/checker/cpu_mce.go`:

```go
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
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/consts"
)

const (
	CPUMCEUncorrectedCheckerName = "cpu-mce-uncorrected"
	CPUMCECorrectedCheckerName   = "cpu-mce-corrected"
)

// CPUMCEUncorrectedChecker checks for uncorrectable MCE events.
type CPUMCEUncorrectedChecker struct {
	name string
}

func NewCPUMCEUncorrectedChecker() (common.Checker, error) {
	return &CPUMCEUncorrectedChecker{name: CPUMCEUncorrectedCheckerName}, nil
}

func (c *CPUMCEUncorrectedChecker) Name() string { return c.name }

func (c *CPUMCEUncorrectedChecker) GetSpec() common.CheckerSpec { return nil }

func (c *CPUMCEUncorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuInfo, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type for CPUMCEUncorrectedChecker")
	}

	result := config.CPUCheckItems[CPUMCEUncorrectedCheckerName]

	if !cpuInfo.MCEInfo.Available {
		result.Status = consts.StatusNormal
		result.Curr = "N/A"
		result.Detail = "MCE monitoring not available on this system"
		result.Level = consts.LevelInfo
		return &result, nil
	}

	result.Curr = fmt.Sprintf("%d", cpuInfo.MCEInfo.UncorrectedCount)
	if cpuInfo.MCEInfo.UncorrectedCount > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Detected %d uncorrectable MCE(s)", cpuInfo.MCEInfo.UncorrectedCount)
	} else {
		result.Status = consts.StatusNormal
	}

	return &result, nil
}

// CPUMCECorrectedChecker checks if correctable MCE count exceeds threshold.
type CPUMCECorrectedChecker struct {
	name      string
	threshold int64
}

func NewCPUMCECorrectedChecker(threshold int64) (common.Checker, error) {
	return &CPUMCECorrectedChecker{
		name:      CPUMCECorrectedCheckerName,
		threshold: threshold,
	}, nil
}

func (c *CPUMCECorrectedChecker) Name() string { return c.name }

func (c *CPUMCECorrectedChecker) GetSpec() common.CheckerSpec { return nil }

func (c *CPUMCECorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuInfo, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid data type for CPUMCECorrectedChecker")
	}

	result := config.CPUCheckItems[CPUMCECorrectedCheckerName]
	result.Spec = fmt.Sprintf("<%d", c.threshold)

	if !cpuInfo.MCEInfo.Available {
		result.Status = consts.StatusNormal
		result.Curr = "N/A"
		result.Detail = "MCE monitoring not available on this system"
		result.Level = consts.LevelInfo
		return &result, nil
	}

	result.Curr = fmt.Sprintf("%d", cpuInfo.MCEInfo.CorrectedCount)
	if cpuInfo.MCEInfo.CorrectedCount >= c.threshold {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Correctable MCE count %d exceeds threshold %d", cpuInfo.MCEInfo.CorrectedCount, c.threshold)
	} else {
		result.Status = consts.StatusNormal
	}

	return &result, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/cpu/checker/ -run TestCPUMCE -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/cpu/checker/cpu_mce.go components/cpu/checker/cpu_mce_test.go
git commit -m "feat(cpu): add MCE uncorrected and corrected checkers"
```

---

### Task 8: Register new CPU checkers in checker factory

**Files:**
- Modify: `components/cpu/checker/checker.go`

- [ ] **Step 1: Add new checkers to NewCheckers()**

Update `components/cpu/checker/checker.go` to register the new checkers. The default thresholds are: clock offset warning 1ms, critical 10ms, MCE CE threshold 10.

Replace the body of `NewCheckers()` to include:

```go
func NewCheckers() ([]common.Checker, error) {
	checkers := make([]common.Checker, 0)

	perfChecker, err := NewCPUPerfChecker()
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, perfChecker)

	syncServiceChecker, err := NewClockSyncServiceChecker()
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, syncServiceChecker)

	syncOffsetChecker, err := NewClockSyncOffsetChecker(1.0, 10.0)
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, syncOffsetChecker)

	mceUncorrectedChecker, err := NewCPUMCEUncorrectedChecker()
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, mceUncorrectedChecker)

	mceCorrectedChecker, err := NewCPUMCECorrectedChecker(10)
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, mceCorrectedChecker)

	return checkers, nil
}
```

- [ ] **Step 2: Verify everything compiles and tests pass**

Run: `go build ./components/cpu/... && go test ./components/cpu/... -v`
Expected: compiles, all tests pass

- [ ] **Step 3: Commit**

```bash
git add components/cpu/checker/checker.go
git commit -m "feat(cpu): register clock sync and MCE checkers in factory"
```

---

### Task 9: Add MCE event rule to CPU event rules

**Files:**
- Modify: `components/cpu/config/default_event_rules.yaml`

- [ ] **Step 1: Add MCE event rule**

Append to the `cpu:` section in `components/cpu/config/default_event_rules.yaml`:

```yaml
  mce_error:
    name: "mce_error"
    description: "Machine Check Exception detected in kernel log"
    log_file: "/var/log/syslog"
    regexp: "(Machine check|MCE|mce:)"
    level: critical
    suggestion: "Check CPU/memory hardware health. Run mcelog or rasdaemon for details"
```

- [ ] **Step 2: Verify CPU component builds**

Run: `go build ./components/cpu/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add components/cpu/config/default_event_rules.yaml
git commit -m "feat(cpu): add MCE event rule for kernel log matching"
```

---

### Task 10: Memory collector — EDAC info

**Files:**
- Create: `components/memory/collector/edac_info.go`
- Create: `components/memory/collector/edac_info_test.go`

- [ ] **Step 1: Write tests**

Create `components/memory/collector/edac_info_test.go`:

```go
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEDACInfoGetFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mc0 with csrow0
	mc0 := filepath.Join(tmpDir, "mc0")
	csrow0 := filepath.Join(mc0, "csrow0")
	require.NoError(t, os.MkdirAll(csrow0, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mc0, "ce_count"), []byte("10\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(mc0, "ue_count"), []byte("0\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow0, "ce_count"), []byte("10\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow0, "ue_count"), []byte("0\n"), 0644))

	// Create mc1 with csrow0
	mc1 := filepath.Join(tmpDir, "mc1")
	csrow1 := filepath.Join(mc1, "csrow0")
	require.NoError(t, os.MkdirAll(csrow1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mc1, "ce_count"), []byte("5\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(mc1, "ue_count"), []byte("2\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow1, "ce_count"), []byte("5\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow1, "ue_count"), []byte("2\n"), 0644))

	info := &EDACInfo{}
	err := info.getFromDir(tmpDir)
	assert.NoError(t, err)
	assert.True(t, info.Available)
	assert.Equal(t, int64(15), info.TotalCE)
	assert.Equal(t, int64(2), info.TotalUCE)
	assert.Len(t, info.Controllers, 2)
}

func TestEDACInfoGetFromDir_NotAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	info := &EDACInfo{}
	err := info.getFromDir(filepath.Join(tmpDir, "nonexistent"))
	assert.NoError(t, err)
	assert.False(t, info.Available)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/memory/collector/ -run TestEDACInfo -v`
Expected: FAIL — `EDACInfo` undefined

- [ ] **Step 3: Implement edac_info.go**

Create `components/memory/collector/edac_info.go`:

```go
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

	"github.com/sirupsen/logrus"
)

const edacSysfsPath = "/sys/devices/system/edac/mc"

// EDACInfo holds EDAC memory controller error counters.
type EDACInfo struct {
	Available   bool     `json:"available"`
	Controllers []MCInfo `json:"controllers"`
	TotalCE     int64    `json:"total_ce"`
	TotalUCE    int64    `json:"total_uce"`
}

// MCInfo holds per-memory-controller error counters.
type MCInfo struct {
	ID       string      `json:"id"`
	CECount  int64       `json:"ce_count"`
	UCECount int64       `json:"uce_count"`
	CSRows   []CSRowInfo `json:"csrows"`
}

// CSRowInfo holds per-chip-select-row error counters.
type CSRowInfo struct {
	ID       string `json:"id"`
	CECount  int64  `json:"ce_count"`
	UCECount int64  `json:"uce_count"`
}

// Get reads EDAC counters from sysfs.
func (e *EDACInfo) Get() error {
	return e.getFromDir(filepath.Dir(edacSysfsPath))
}

// getFromDir reads EDAC counters from a given base directory (testable).
// It expects mc* subdirectories under baseDir.
func (e *EDACInfo) getFromDir(baseDir string) error {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		logrus.WithField("collector", "edac_info").Debugf("EDAC sysfs not available at %s: %v", baseDir, err)
		e.Available = false
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "mc") {
			continue
		}
		mcDir := filepath.Join(baseDir, entry.Name())
		mc := MCInfo{
			ID:       entry.Name(),
			CECount:  readEdacIntFile(filepath.Join(mcDir, "ce_count")),
			UCECount: readEdacIntFile(filepath.Join(mcDir, "ue_count")),
		}

		// Read csrow subdirectories
		csEntries, err := os.ReadDir(mcDir)
		if err == nil {
			for _, csEntry := range csEntries {
				if !csEntry.IsDir() || !strings.HasPrefix(csEntry.Name(), "csrow") {
					continue
				}
				csDir := filepath.Join(mcDir, csEntry.Name())
				mc.CSRows = append(mc.CSRows, CSRowInfo{
					ID:       csEntry.Name(),
					CECount:  readEdacIntFile(filepath.Join(csDir, "ce_count")),
					UCECount: readEdacIntFile(filepath.Join(csDir, "ue_count")),
				})
			}
		}

		e.Controllers = append(e.Controllers, mc)
		e.TotalCE += mc.CECount
		e.TotalUCE += mc.UCECount
	}

	e.Available = len(e.Controllers) > 0
	return nil
}

func readEdacIntFile(path string) int64 {
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/memory/collector/ -run TestEDACInfo -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/memory/collector/edac_info.go components/memory/collector/edac_info_test.go
git commit -m "feat(memory): add EDAC sysfs collector"
```

---

### Task 11: Memory collector — capacity info

**Files:**
- Create: `components/memory/collector/memory_capacity.go`
- Create: `components/memory/collector/memory_capacity_test.go`

- [ ] **Step 1: Write tests**

Create `components/memory/collector/memory_capacity_test.go`:

```go
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemoryCapacityInfo(t *testing.T) {
	// MemoryInfo.MemTotal is in bytes (parsed from /proc/meminfo kB * 1024)
	memInfo := &MemoryInfo{MemTotal: 1073741824} // 1 GB in bytes

	cap := MemoryCapacityFromMemInfo(memInfo)
	assert.Equal(t, int64(1073741824), cap.TotalBytes)
	assert.InDelta(t, 1.0, cap.TotalGB, 0.01)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/memory/collector/ -run TestMemoryCapacity -v`
Expected: FAIL — `MemoryCapacityFromMemInfo` undefined

- [ ] **Step 3: Implement memory_capacity.go**

Create `components/memory/collector/memory_capacity.go`:

```go
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

// MemoryCapacityInfo holds memory capacity data.
type MemoryCapacityInfo struct {
	TotalBytes int64   `json:"total_bytes"`
	TotalGB    float64 `json:"total_gb"`
}

// MemoryCapacityFromMemInfo derives capacity from MemoryInfo.
func MemoryCapacityFromMemInfo(m *MemoryInfo) MemoryCapacityInfo {
	totalBytes := int64(m.MemTotal)
	return MemoryCapacityInfo{
		TotalBytes: totalBytes,
		TotalGB:    float64(totalBytes) / (1024 * 1024 * 1024),
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/memory/collector/ -run TestMemoryCapacity -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/memory/collector/memory_capacity.go components/memory/collector/memory_capacity_test.go
git commit -m "feat(memory): add memory capacity collector"
```

---

### Task 12: Extend Memory collector Output with EDAC + capacity

**Files:**
- Modify: `components/memory/collector/collector.go`

- [ ] **Step 1: Add EDAC and Capacity fields to Output**

In `components/memory/collector/collector.go`, add the new fields to the `Output` struct:

```go
type Output struct {
	Info     *MemoryInfo        `json:"info"`
	EDAC     EDACInfo           `json:"edac"`
	Capacity MemoryCapacityInfo `json:"capacity"`
	Time     time.Time
}
```

- [ ] **Step 2: Collect EDAC and Capacity in Collect()**

In the `Collect` method, after the existing `memInfo.Get()` call, add EDAC and capacity collection. Add these lines before the return:

```go
	edac := &EDACInfo{}
	if err := edac.Get(); err != nil {
		logrus.WithField("collector", "memory").Warnf("failed to collect EDAC info: %v", err)
	}

	capacity := MemoryCapacityFromMemInfo(memInfo)

	output := &Output{
		Info:     memInfo,
		EDAC:     *edac,
		Capacity: capacity,
		Time:     time.Now(),
	}
```

Make sure to update the existing code that creates the `Output` to include the new fields. The existing `Collect()` returns `&Output{Info: memInfo, Time: time.Now()}` — replace that construction with the version above.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./components/memory/...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add components/memory/collector/collector.go
git commit -m "feat(memory): extend collector output with EDAC and capacity info"
```

---

### Task 13: Memory config — add check items

**Files:**
- Create: `components/memory/config/check_items.go`

- [ ] **Step 1: Create check_items.go**

Create `components/memory/config/check_items.go`:

```go
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
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

var MemoryCheckItems = map[string]common.CheckerResult{
	"memory-ecc-uncorrected": {
		Name:        "memory-ecc-uncorrected",
		Description: "Check for uncorrectable memory ECC errors",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "MemoryECCUncorrected",
		Suggestion:  "Uncorrectable memory error detected. Identify faulty DIMM and replace",
	},
	"memory-ecc-corrected": {
		Name:        "memory-ecc-corrected",
		Description: "Check correctable memory ECC error count is below threshold",
		Spec:        "<100",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "MemoryECCCorrectedHigh",
		Suggestion:  "Correctable memory errors increasing. Monitor DIMM health and plan replacement",
	},
	"memory-capacity": {
		Name:        "memory-capacity",
		Description: "Check total memory matches expected specification",
		Spec:        "matches spec",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "MemoryCapacityMismatch",
		Suggestion:  "Memory capacity does not match spec. Check for failed DIMMs",
	},
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./components/memory/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add components/memory/config/check_items.go
git commit -m "feat(memory): add check item templates for ECC and capacity checkers"
```

---

### Task 14: Memory checker — ECC

**Files:**
- Create: `components/memory/checker/memory_ecc.go`
- Create: `components/memory/checker/memory_ecc_test.go`

- [ ] **Step 1: Write tests**

Create `components/memory/checker/memory_ecc_test.go`:

```go
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
package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryECCUncorrectedChecker(t *testing.T) {
	tests := []struct {
		name       string
		edac       collector.EDACInfo
		wantStatus string
	}{
		{
			name:       "no UCE",
			edac:       collector.EDACInfo{Available: true, TotalUCE: 0},
			wantStatus: consts.StatusNormal,
		},
		{
			name:       "UCE detected",
			edac:       collector.EDACInfo{Available: true, TotalUCE: 3},
			wantStatus: consts.StatusAbnormal,
		},
		{
			name:       "EDAC not available",
			edac:       collector.EDACInfo{Available: false},
			wantStatus: consts.StatusNormal,
		},
	}

	checker, err := NewMemoryECCUncorrectedChecker()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &collector.Output{
				Info: &collector.MemoryInfo{},
				EDAC: tt.edac,
			}
			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}

func TestMemoryECCCorrectedChecker(t *testing.T) {
	tests := []struct {
		name       string
		edac       collector.EDACInfo
		threshold  int64
		wantStatus string
	}{
		{
			name:       "CE below threshold",
			edac:       collector.EDACInfo{Available: true, TotalCE: 50},
			threshold:  100,
			wantStatus: consts.StatusNormal,
		},
		{
			name:       "CE above threshold",
			edac:       collector.EDACInfo{Available: true, TotalCE: 150},
			threshold:  100,
			wantStatus: consts.StatusAbnormal,
		},
		{
			name:       "EDAC not available",
			edac:       collector.EDACInfo{Available: false},
			threshold:  100,
			wantStatus: consts.StatusNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewMemoryECCCorrectedChecker(tt.threshold)
			require.NoError(t, err)
			data := &collector.Output{
				Info: &collector.MemoryInfo{},
				EDAC: tt.edac,
			}
			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/memory/checker/ -run TestMemoryECC -v`
Expected: FAIL — package has no Go files / types undefined

- [ ] **Step 3: Implement memory_ecc.go**

Create `components/memory/checker/memory_ecc.go`:

```go
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
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/components/memory/config"
	"github.com/scitix/sichek/consts"
)

const (
	MemoryECCUncorrectedCheckerName = "memory-ecc-uncorrected"
	MemoryECCCorrectedCheckerName   = "memory-ecc-corrected"
)

// MemoryECCUncorrectedChecker checks for uncorrectable ECC errors.
type MemoryECCUncorrectedChecker struct {
	name string
}

func NewMemoryECCUncorrectedChecker() (common.Checker, error) {
	return &MemoryECCUncorrectedChecker{name: MemoryECCUncorrectedCheckerName}, nil
}

func (c *MemoryECCUncorrectedChecker) Name() string { return c.name }

func (c *MemoryECCUncorrectedChecker) GetSpec() common.CheckerSpec { return nil }

func (c *MemoryECCUncorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	memOutput, ok := data.(*collector.Output)
	if !ok {
		return nil, fmt.Errorf("invalid data type for MemoryECCUncorrectedChecker")
	}

	result := config.MemoryCheckItems[MemoryECCUncorrectedCheckerName]

	if !memOutput.EDAC.Available {
		result.Status = consts.StatusNormal
		result.Curr = "N/A"
		result.Detail = "EDAC not available. ECC may not be enabled"
		result.Level = consts.LevelInfo
		return &result, nil
	}

	result.Curr = fmt.Sprintf("%d", memOutput.EDAC.TotalUCE)
	if memOutput.EDAC.TotalUCE > 0 {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Detected %d uncorrectable memory ECC error(s)", memOutput.EDAC.TotalUCE)
	} else {
		result.Status = consts.StatusNormal
	}

	return &result, nil
}

// MemoryECCCorrectedChecker checks if correctable ECC count exceeds threshold.
type MemoryECCCorrectedChecker struct {
	name      string
	threshold int64
}

func NewMemoryECCCorrectedChecker(threshold int64) (common.Checker, error) {
	return &MemoryECCCorrectedChecker{
		name:      MemoryECCCorrectedCheckerName,
		threshold: threshold,
	}, nil
}

func (c *MemoryECCCorrectedChecker) Name() string { return c.name }

func (c *MemoryECCCorrectedChecker) GetSpec() common.CheckerSpec { return nil }

func (c *MemoryECCCorrectedChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	memOutput, ok := data.(*collector.Output)
	if !ok {
		return nil, fmt.Errorf("invalid data type for MemoryECCCorrectedChecker")
	}

	result := config.MemoryCheckItems[MemoryECCCorrectedCheckerName]
	result.Spec = fmt.Sprintf("<%d", c.threshold)

	if !memOutput.EDAC.Available {
		result.Status = consts.StatusNormal
		result.Curr = "N/A"
		result.Detail = "EDAC not available. ECC may not be enabled"
		result.Level = consts.LevelInfo
		return &result, nil
	}

	result.Curr = fmt.Sprintf("%d", memOutput.EDAC.TotalCE)
	if memOutput.EDAC.TotalCE >= c.threshold {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Correctable ECC count %d exceeds threshold %d", memOutput.EDAC.TotalCE, c.threshold)
	} else {
		result.Status = consts.StatusNormal
	}

	return &result, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/memory/checker/ -run TestMemoryECC -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/memory/checker/memory_ecc.go components/memory/checker/memory_ecc_test.go
git commit -m "feat(memory): add ECC uncorrected and corrected checkers"
```

---

### Task 15: Memory checker — capacity

**Files:**
- Create: `components/memory/checker/memory_capacity.go`
- Create: `components/memory/checker/memory_capacity_test.go`

- [ ] **Step 1: Write tests**

Create `components/memory/checker/memory_capacity_test.go`:

```go
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
package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCapacityChecker(t *testing.T) {
	tests := []struct {
		name        string
		totalGB     float64
		expectedGB  float64
		tolerancePct float64
		wantStatus  string
	}{
		{
			name:        "capacity matches",
			totalGB:     1024.0,
			expectedGB:  1024.0,
			tolerancePct: 5.0,
			wantStatus:  consts.StatusNormal,
		},
		{
			name:        "capacity within tolerance",
			totalGB:     1000.0,
			expectedGB:  1024.0,
			tolerancePct: 5.0,
			wantStatus:  consts.StatusNormal,
		},
		{
			name:        "capacity below tolerance",
			totalGB:     900.0,
			expectedGB:  1024.0,
			tolerancePct: 5.0,
			wantStatus:  consts.StatusAbnormal,
		},
		{
			name:        "no expected value — skip",
			totalGB:     512.0,
			expectedGB:  0,
			tolerancePct: 5.0,
			wantStatus:  consts.StatusNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewMemoryCapacityChecker(tt.expectedGB, tt.tolerancePct)
			require.NoError(t, err)

			totalBytes := int64(tt.totalGB * 1024 * 1024 * 1024)
			data := &collector.Output{
				Info:     &collector.MemoryInfo{MemTotal: int(totalBytes)},
				Capacity: collector.MemoryCapacityInfo{TotalBytes: totalBytes, TotalGB: tt.totalGB},
			}
			result, err := checker.Check(context.Background(), data)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, result.Status)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./components/memory/checker/ -run TestMemoryCapacity -v`
Expected: FAIL — `NewMemoryCapacityChecker` undefined

- [ ] **Step 3: Implement memory_capacity.go**

Create `components/memory/checker/memory_capacity.go`:

```go
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
package checker

import (
	"context"
	"fmt"
	"math"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/components/memory/config"
	"github.com/scitix/sichek/consts"
)

const MemoryCapacityCheckerName = "memory-capacity"

// MemoryCapacityChecker checks if total memory matches the expected spec.
type MemoryCapacityChecker struct {
	name         string
	expectedGB   float64
	tolerancePct float64
}

func NewMemoryCapacityChecker(expectedGB, tolerancePct float64) (common.Checker, error) {
	return &MemoryCapacityChecker{
		name:         MemoryCapacityCheckerName,
		expectedGB:   expectedGB,
		tolerancePct: tolerancePct,
	}, nil
}

func (c *MemoryCapacityChecker) Name() string { return c.name }

func (c *MemoryCapacityChecker) GetSpec() common.CheckerSpec { return nil }

func (c *MemoryCapacityChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	memOutput, ok := data.(*collector.Output)
	if !ok {
		return nil, fmt.Errorf("invalid data type for MemoryCapacityChecker")
	}

	result := config.MemoryCheckItems[MemoryCapacityCheckerName]

	if c.expectedGB <= 0 {
		result.Status = consts.StatusNormal
		result.Curr = fmt.Sprintf("%.1fGB", memOutput.Capacity.TotalGB)
		result.Detail = "No expected capacity in spec, skipping check"
		return &result, nil
	}

	result.Spec = fmt.Sprintf("%.0fGB (±%.0f%%)", c.expectedGB, c.tolerancePct)
	result.Curr = fmt.Sprintf("%.1fGB", memOutput.Capacity.TotalGB)

	diffPct := math.Abs(memOutput.Capacity.TotalGB-c.expectedGB) / c.expectedGB * 100
	if diffPct > c.tolerancePct {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Memory %.1fGB differs from expected %.0fGB by %.1f%% (tolerance: %.0f%%)",
			memOutput.Capacity.TotalGB, c.expectedGB, diffPct, c.tolerancePct)
	} else {
		result.Status = consts.StatusNormal
	}

	return &result, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./components/memory/checker/ -run TestMemoryCapacity -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add components/memory/checker/memory_capacity.go components/memory/checker/memory_capacity_test.go
git commit -m "feat(memory): add memory capacity checker"
```

---

### Task 16: Memory checker factory

**Files:**
- Create: `components/memory/checker/checker.go`

- [ ] **Step 1: Create checker factory**

Create `components/memory/checker/checker.go`:

```go
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
package checker

import (
	"github.com/scitix/sichek/components/common"
)

// NewCheckers creates all memory checkers with default thresholds.
// Default thresholds: ECC CE threshold = 100, expected capacity = 0 (skip), tolerance = 5%.
func NewCheckers(expectedCapacityGB float64) ([]common.Checker, error) {
	checkers := make([]common.Checker, 0)

	eccUncorrected, err := NewMemoryECCUncorrectedChecker()
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, eccUncorrected)

	eccCorrected, err := NewMemoryECCCorrectedChecker(100)
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, eccCorrected)

	capacityChecker, err := NewMemoryCapacityChecker(expectedCapacityGB, 5.0)
	if err != nil {
		return nil, err
	}
	checkers = append(checkers, capacityChecker)

	return checkers, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./components/memory/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add components/memory/checker/checker.go
git commit -m "feat(memory): add checker factory for ECC and capacity checkers"
```

---

### Task 17: Register memory checkers in memory component

**Files:**
- Modify: `components/memory/memory.go`

- [ ] **Step 1: Add checker import and registration**

In `components/memory/memory.go`, add the checker import:

```go
import (
	// ... existing imports ...
	"github.com/scitix/sichek/components/memory/checker"
)
```

In `newMemoryComponent()`, after the collector creation and before the filter creation, add:

```go
	checkers, err := checker.NewCheckers(0) // 0 = no expected capacity, skip capacity check by default
	if err != nil {
		logrus.WithField("component", "memory").Errorf("NewMemoryComponent create checkers failed: %v", err)
		return nil, err
	}
```

Then change `checkers: nil` to `checkers: checkers` in the component struct initialization.

- [ ] **Step 2: Verify it compiles and tests pass**

Run: `go build ./components/memory/... && go test ./components/memory/... -v`
Expected: compiles, all tests pass

- [ ] **Step 3: Commit**

```bash
git add components/memory/memory.go
git commit -m "feat(memory): register ECC and capacity checkers in memory component"
```

---

### Task 18: Add EDAC event rule to memory event rules

**Files:**
- Modify: `components/memory/config/default_event_rules.yaml`

- [ ] **Step 1: Verify existing rules and add EDAC event rule**

The file already has `UncorrectableECC` and `CorrectableECC` rules matching EDAC patterns in syslog. These cover log-based detection. Review the existing rules — they use `(EDAC MC0)&(UE memory)` and `(EDAC MC0)&(CE memory)` patterns.

The existing rules are already sufficient for log-based EDAC event detection. No changes needed to this file.

- [ ] **Step 2: Commit** (skip if no changes)

No commit needed — existing event rules already cover EDAC log matching.

---

### Task 19: Full integration test

**Files:**
- No new files, testing existing code

- [ ] **Step 1: Build the entire project**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 2: Run all CPU tests**

Run: `go test ./components/cpu/... -v`
Expected: all tests pass

- [ ] **Step 3: Run all memory tests**

Run: `go test ./components/memory/... -v`
Expected: all tests pass

- [ ] **Step 4: Run full test suite**

Run: `go test ./...`
Expected: all tests pass (or only pre-existing failures)

- [ ] **Step 5: Verify go vet passes**

Run: `go vet ./...`
Expected: no new warnings

---

### Summary of all new/modified files

**New files (14):**
- `components/cpu/collector/ptp_info.go`
- `components/cpu/collector/ptp_info_test.go`
- `components/cpu/collector/mce_info.go`
- `components/cpu/collector/mce_info_test.go`
- `components/cpu/checker/clock_sync.go`
- `components/cpu/checker/clock_sync_test.go`
- `components/cpu/checker/cpu_mce.go`
- `components/cpu/checker/cpu_mce_test.go`
- `components/memory/collector/edac_info.go`
- `components/memory/collector/edac_info_test.go`
- `components/memory/collector/memory_capacity.go`
- `components/memory/collector/memory_capacity_test.go`
- `components/memory/checker/memory_ecc.go`
- `components/memory/checker/memory_ecc_test.go`
- `components/memory/checker/memory_capacity.go`
- `components/memory/checker/memory_capacity_test.go`
- `components/memory/checker/checker.go`
- `components/memory/config/check_items.go`

**Modified files (5):**
- `consts/consts.go`
- `components/cpu/collector/collector.go`
- `components/cpu/config/check_items.go`
- `components/cpu/checker/checker.go`
- `components/cpu/config/default_event_rules.yaml`
- `components/memory/collector/collector.go`
- `components/memory/memory.go`
