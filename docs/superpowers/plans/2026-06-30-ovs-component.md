# sichek `ovs` Component Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a native Go `ovs` component to sichek that health-checks Spectrum-X DOCA-OVS data plane, lands results in the snapshot + K8s annotation, and exports Prometheus metrics mirroring the rdma-doctor OVS schema.

**Architecture:** Mirror `components/transceiver/`. A `collector` shells out to `ovs-vsctl`/`ovs-ofctl`/`ovs-appctl`; four `checker`s compare against `default_ovs_spec.yaml`; a `metrics` package exports `sichek_ovs_*` gauges. The component auto-skips on nodes without DOCA-OVS, so it can safely join `DefaultComponents`. No dependency on `rdma_env_vv` or rdma-doctor at runtime.

**Tech Stack:** Go 1.23, `os/exec`, `testify`, `prometheus/client_golang` (via `metrics.GaugeVecMetricExporter`), cobra.

**Spec:** `docs/superpowers/specs/2026-06-30-ovs-component-design.md`

**Conventions:** Every new `.go` file starts with the Apache-2.0 Scitix copyright header (see CLAUDE.md). Commit messages omit any Claude co-author trailer. Run `go test ./components/ovs/...` from repo root.

---

## File Structure

```
components/ovs/
  ovs.go                              # component: implements common.Component
  collector/
    collector.go                      # OVSCollector + Collect() + availability gate
    ovs_info.go                       # OVSInfo struct (common.Info) + sub-structs
    parse.go                          # pure parse functions (no exec)
    parse_test.go                     # table-driven parser tests (fixtures)
    testdata/                         # captured ovs-* outputs
  checker/
    checker.go                        # NewCheckers()
    service_checker.go
    version_checker.go
    other_config_checker.go
    bridge_checker.go
    checker_test.go                   # table-driven checker tests
  config/
    config.go                         # OVSUserConfig
    spec.go                           # OVSSpec + LoadSpec + built-in default
    default_ovs_user_config.yaml
    default_ovs_spec.yaml
  metrics/
    metrics.go                        # OVSMetrics, prefix sichek_ovs
cmd/command/component/ovs.go          # cobra subcommand
```

**Edits to existing files:**
- `consts/consts.go` — add `ComponentNameOVS`, append to `DefaultComponents`
- `cmd/command/component/all.go` — import + `NewComponent` switch case
- `cmd/command/command.go` — `rootCmd.AddCommand(component.NewOVSCmd())`
- `service/info.go` — `nodeAnnotation.OVS` field + `getAnnotationsByItem` + `setAnnotationsByItem` cases

---

## Task 1: Register component name + consts

**Files:**
- Modify: `consts/consts.go`

- [ ] **Step 1: Add the component name constant**

In `consts/consts.go`, in the `ComponentName*` block (after `ComponentNameLLDP = "lldp"` at line ~49), add:

```go
	ComponentNameOVS          = "ovs"
```

- [ ] **Step 2: Append to DefaultComponents**

In the `DefaultComponents` slice (line ~99), add `ComponentNameOVS` to the list:

```go
	DefaultComponents = []string{
		ComponentNameCPU, ComponentNameNvidia, ComponentNameInfiniband, ComponentNameEthernet, ComponentNameGpfs, ComponentNameDmesg,
		ComponentNamePodlog, ComponentNameGpuEvents, ComponentNameSyslog, ComponentNameTransceiver, ComponentNameLLDP,
		ComponentNameOVS,
	}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./consts/...`
Expected: no output (success).

- [ ] **Step 4: Commit**

```bash
git add consts/consts.go
git commit -m "feat(ovs): register ovs component name + default component"
```

---

## Task 2: Config package (user config + spec)

**Files:**
- Create: `components/ovs/config/config.go`
- Create: `components/ovs/config/spec.go`
- Create: `components/ovs/config/default_ovs_user_config.yaml`
- Create: `components/ovs/config/default_ovs_spec.yaml`
- Test: `components/ovs/config/spec_test.go`

- [ ] **Step 1: Write `config.go`**

```go
// <Apache header>
package config

import "github.com/scitix/sichek/components/common"

const (
	ServiceCheckerName     = "ovs_service"
	VersionCheckerName     = "ovs_version"
	OtherConfigCheckerName = "ovs_other_config"
	BridgeCheckerName      = "ovs_bridge"
)

type OVSUserConfig struct {
	OVS *OVSConfig `json:"ovs" yaml:"ovs"`
}

type OVSConfig struct {
	QueryInterval   common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64           `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers []string        `json:"ignored_checkers" yaml:"ignored_checkers"`
	EnableMetrics   bool            `json:"enable_metrics" yaml:"enable_metrics"`
}

func (c *OVSUserConfig) GetQueryInterval() common.Duration {
	if c.OVS == nil {
		return common.Duration{}
	}
	return c.OVS.QueryInterval
}

func (c *OVSUserConfig) SetQueryInterval(newInterval common.Duration) {
	if c.OVS == nil {
		c.OVS = &OVSConfig{}
	}
	c.OVS.QueryInterval = newInterval
}
```

- [ ] **Step 2: Write `spec.go`**

The spec loader is intentionally simpler than transceiver's (no per-device OSS download — there is one baseline). It loads the given YAML file, else returns the built-in default.

```go
// <Apache header>
package config

import (
	"github.com/scitix/sichek/components/common"
	"github.com/sirupsen/logrus"
)

type OVSSpecConfig struct {
	OVS *OVSSpec `json:"ovs" yaml:"ovs"`
}

type OVSSpec struct {
	BridgePrefix      string            `json:"bridge_prefix" yaml:"bridge_prefix"`
	NumRails          int               `json:"num_rails" yaml:"num_rails"`
	PortsPerBridge    int               `json:"ports_per_bridge" yaml:"ports_per_bridge"`
	MinFlows          int               `json:"min_flows" yaml:"min_flows"`
	ExpectedGroupIDs  []int             `json:"expected_group_ids" yaml:"expected_group_ids"`
	DatapathType      string            `json:"datapath_type" yaml:"datapath_type"`
	OtherConfig       map[string]string `json:"other_config" yaml:"other_config"`
	RequiredPackages  []string          `json:"required_packages" yaml:"required_packages"`
	CoverageEvents    []string          `json:"coverage_events" yaml:"coverage_events"`
}

// LoadSpec loads the OVS spec from file; on any failure it returns the built-in default.
func LoadSpec(file string) (*OVSSpec, error) {
	if file == "" {
		return DefaultSpec(), nil
	}
	var s OVSSpecConfig
	if err := common.LoadSpec(file, &s); err != nil || s.OVS == nil {
		logrus.WithField("component", "ovs/spec").Warnf("LoadSpec %s failed or empty, using built-in default: %v", file, err)
		return DefaultSpec(), nil
	}
	return s.OVS, nil
}

// DefaultSpec mirrors the rdma_env_vv ovs hardcoded baselines (Step 5/8/10).
func DefaultSpec() *OVSSpec {
	return &OVSSpec{
		BridgePrefix:     "br-rail",
		NumRails:         8,
		PortsPerBridge:   5,
		MinFlows:         18,
		ExpectedGroupIDs: []int{10, 20, 21, 22, 23, 30, 31, 32, 33},
		DatapathType:     "netdev",
		OtherConfig: map[string]string{
			"doca-init":          "true",
			"hw-offload":         "true",
			"hw-offload-ct-size": "0",
			"max-idle":           "300000",
			"doca-eswitch-max":   "4",
		},
		RequiredPackages: []string{
			"doca-openvswitch-switch",
			"doca-openvswitch-common",
			"collectx-clxapi",
			"libnvhws1",
		},
		CoverageEvents: []string{
			"flow_offload_200ms_latency",
			"doca_datapath_drop_upcall_error",
		},
	}
}
```

> Note: if `common.LoadSpec(file, &s)` signature differs (it takes `(file string, out interface{}) error` as used in transceiver `spec.go:67`), keep this call shape. Confirm by reading `components/common` spec helpers before writing.

- [ ] **Step 3: Write `default_ovs_spec.yaml`**

```yaml
ovs:
  bridge_prefix: "br-rail"
  num_rails: 8
  ports_per_bridge: 5
  min_flows: 18
  expected_group_ids: [10, 20, 21, 22, 23, 30, 31, 32, 33]
  datapath_type: "netdev"
  other_config:
    doca-init: "true"
    hw-offload: "true"
    hw-offload-ct-size: "0"
    max-idle: "300000"
    doca-eswitch-max: "4"
  required_packages:
    - doca-openvswitch-switch
    - doca-openvswitch-common
    - collectx-clxapi
    - libnvhws1
  coverage_events:
    - flow_offload_200ms_latency
    - doca_datapath_drop_upcall_error
```

- [ ] **Step 4: Write `default_ovs_user_config.yaml`**

```yaml
ovs:
  query_interval: 60s
  cache_size: 5
  enable_metrics: true
  ignored_checkers: []
```

- [ ] **Step 5: Write `spec_test.go`**

```go
// <Apache header>
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadSpec_EmptyFileReturnsDefault(t *testing.T) {
	spec, err := LoadSpec("")
	assert.NoError(t, err)
	assert.Equal(t, 8, spec.NumRails)
	assert.Equal(t, 5, spec.PortsPerBridge)
	assert.Equal(t, 18, spec.MinFlows)
	assert.Equal(t, []int{10, 20, 21, 22, 23, 30, 31, 32, 33}, spec.ExpectedGroupIDs)
	assert.Equal(t, "true", spec.OtherConfig["hw-offload"])
	assert.Contains(t, spec.RequiredPackages, "libnvhws1")
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./components/ovs/config/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add components/ovs/config/
git commit -m "feat(ovs): config + spec with rdma_env_vv baselines"
```

---

## Task 3: Collector data model (`OVSInfo`)

**Files:**
- Create: `components/ovs/collector/ovs_info.go`

- [ ] **Step 1: Write `ovs_info.go`**

```go
// <Apache header>
package collector

import (
	"encoding/json"
	"time"
)

type OVSInfo struct {
	Time            time.Time         `json:"time"`
	Available       bool              `json:"available"`
	SkipReason      string            `json:"skip_reason,omitempty"`
	Services        map[string]string `json:"services"`         // svc -> systemctl state
	Packages        map[string]string `json:"packages"`         // pkg -> version ("" = not installed)
	OVSVersion      string            `json:"ovs_version"`
	DPDKVersion     string            `json:"dpdk_version"`
	DPDKInitialized bool              `json:"dpdk_initialized"`
	OtherConfig     map[string]string `json:"other_config"`
	Bridges         []BridgeInfo      `json:"bridges"`
	Datapath        DatapathInfo      `json:"datapath"`
	Coverage        map[string]uint64 `json:"coverage"`
}

type BridgeInfo struct {
	Name           string     `json:"name"`
	Exists         bool       `json:"exists"`
	DatapathType   string     `json:"datapath_type"`
	FailMode       string     `json:"fail_mode"`
	Ports          int        `json:"ports"`
	Flows          int        `json:"flows"`
	GroupIDs       []int      `json:"group_ids"`
	OrphanFlowRefs []int      `json:"orphan_flow_refs"`
	OrphanPorts    []int      `json:"orphan_ports"`
	PortDetails    []PortInfo `json:"port_details"`
}

type PortInfo struct {
	Name       string `json:"name"`
	OFPort     int    `json:"ofport"`
	AdminState string `json:"admin_state"`
	LinkState  string `json:"link_state"`
	Error      string `json:"error,omitempty"`
	RxBytes    uint64 `json:"rx_bytes"`
	TxBytes    uint64 `json:"tx_bytes"`
	RxErrPkts  uint64 `json:"rx_err_pkts"`
}

type DatapathInfo struct {
	Name          string    `json:"name"`
	DPFlows       int       `json:"dp_flows"`
	LookupsHit    uint64    `json:"lookups_hit"`
	LookupsMissed uint64    `json:"lookups_missed"`
	LookupsLost   uint64    `json:"lookups_lost"`
	PMDs          []PMDInfo `json:"pmds"`
}

type PMDInfo struct {
	Core             string  `json:"core"`
	NUMA             string  `json:"numa"`
	BusyRatio        float64 `json:"busy_ratio"`
	IdleCycles       uint64  `json:"idle_cycles"`
	ProcessingCycles uint64  `json:"processing_cycles"`
	RxPackets        uint64  `json:"rx_packets"`
}

// JSON satisfies common.Info.
func (o *OVSInfo) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./components/ovs/collector/...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add components/ovs/collector/ovs_info.go
git commit -m "feat(ovs): OVSInfo collector data model"
```

---

## Task 4: Capture test fixtures from a live node

**Files:**
- Create: `components/ovs/collector/testdata/*.txt`

- [ ] **Step 1: Capture real command outputs from zy3 (healthy node)**

Run (from a shell with ssh access to zy3):

```bash
mkdir -p components/ovs/collector/testdata
ssh zy3 'ovs-ofctl dump-flows br-rail0'        > components/ovs/collector/testdata/dump-flows_br-rail0.txt
ssh zy3 'ovs-ofctl dump-groups br-rail0'       > components/ovs/collector/testdata/dump-groups_br-rail0.txt
ssh zy3 'ovs-ofctl show br-rail0'              > components/ovs/collector/testdata/ofctl-show_br-rail0.txt
ssh zy3 'ovs-appctl dpctl/show'                > components/ovs/collector/testdata/dpctl-show.txt
ssh zy3 'ovs-appctl dpif-netdev/pmd-perf-show' > components/ovs/collector/testdata/pmd-perf-show.txt
ssh zy3 'ovs-appctl coverage/show'             > components/ovs/collector/testdata/coverage-show.txt
```

These are the inputs for the parser tests in Task 5. Inspect each so the parser code below matches the real shapes.

- [ ] **Step 2: Commit**

```bash
git add components/ovs/collector/testdata/
git commit -m "test(ovs): capture live ovs command fixtures from zy3"
```

---

## Task 5: Pure parse functions (TDD)

**Files:**
- Create: `components/ovs/collector/parse.go`
- Test: `components/ovs/collector/parse_test.go`

These functions take raw command output strings and return parsed structures. They contain **no exec calls** so they are unit-testable from fixtures.

- [ ] **Step 1: Write failing tests in `parse_test.go`**

```go
// <Apache header>
package collector

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return string(b)
}

func TestParseFlowCount(t *testing.T) {
	out := readFixture(t, "dump-flows_br-rail0.txt")
	// rdma_env_vv counts lines containing "cookie="
	assert.GreaterOrEqual(t, parseFlowCount(out), 18)
}

func TestParseGroupIDs(t *testing.T) {
	out := readFixture(t, "dump-groups_br-rail0.txt")
	ids := parseGroupIDs(out)
	for _, want := range []int{10, 20, 21, 22, 23, 30, 31, 32, 33} {
		assert.Contains(t, ids, want)
	}
}

func TestParseOFPortsAndRefs(t *testing.T) {
	show := readFixture(t, "ofctl-show_br-rail0.txt")
	flows := readFixture(t, "dump-flows_br-rail0.txt")
	ports := parseOFShowPorts(show)         // ofport numbers present on the bridge
	refs := parseFlowPortRefs(flows)        // in_port=/output: numbers referenced by flows
	orphanRefs, orphanPorts := diffPorts(ports, refs)
	assert.Empty(t, orphanRefs, "healthy node should have no orphan flow refs")
	_ = orphanPorts
}

func TestParseDatapathLookups(t *testing.T) {
	out := readFixture(t, "dpctl-show.txt")
	dp := parseDatapath(out)
	assert.NotEmpty(t, dp.Name)
	assert.Greater(t, dp.LookupsHit, uint64(0))
}

func TestParseCoverage(t *testing.T) {
	out := readFixture(t, "coverage-show.txt")
	cov := parseCoverage(out, []string{"flow_offload_200ms_latency", "doca_datapath_drop_upcall_error"})
	_, ok := cov["doca_datapath_drop_upcall_error"]
	assert.True(t, ok)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./components/ovs/collector/... -run TestParse -v`
Expected: FAIL — `undefined: parseFlowCount` etc.

- [ ] **Step 3: Implement `parse.go`**

```go
// <Apache header>
package collector

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reGroupID   = regexp.MustCompile(`group_id=(\d+)`)
	reOFShow    = regexp.MustCompile(`^\s+(\d+)\(`)
	rePortRef   = regexp.MustCompile(`(?:in_port=|output:)(\d+)`)
	reCoverage  = regexp.MustCompile(`^(\S+)\s.*total:\s*(\d+)`)
)

// parseFlowCount mirrors `dump-flows | grep -c cookie=`.
func parseFlowCount(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "cookie=") {
			n++
		}
	}
	return n
}

// parseGroupIDs returns the sorted-unique set of group_id values present.
func parseGroupIDs(out string) []int {
	seen := map[int]bool{}
	for _, m := range reGroupID.FindAllStringSubmatch(out, -1) {
		if v, err := strconv.Atoi(m[1]); err == nil {
			seen[v] = true
		}
	}
	return keysSorted(seen)
}

// parseOFShowPorts returns numeric ofport ids from `ovs-ofctl show`.
func parseOFShowPorts(out string) []int {
	seen := map[int]bool{}
	for _, line := range strings.Split(out, "\n") {
		if m := reOFShow.FindStringSubmatch(line); m != nil {
			if v, err := strconv.Atoi(m[1]); err == nil {
				seen[v] = true
			}
		}
	}
	return keysSorted(seen)
}

// parseFlowPortRefs returns numeric ports referenced by in_port=/output: in flows.
func parseFlowPortRefs(out string) []int {
	seen := map[int]bool{}
	for _, m := range rePortRef.FindAllStringSubmatch(out, -1) {
		if v, err := strconv.Atoi(m[1]); err == nil {
			seen[v] = true
		}
	}
	return keysSorted(seen)
}

// diffPorts returns (refs not present as ports, ports never referenced by a flow).
func diffPorts(ports, refs []int) (orphanRefs, orphanPorts []int) {
	pset := toSet(ports)
	rset := toSet(refs)
	for _, r := range refs {
		if !pset[r] {
			orphanRefs = append(orphanRefs, r)
		}
	}
	for _, p := range ports {
		if !rset[p] {
			orphanPorts = append(orphanPorts, p)
		}
	}
	return
}

// parseDatapath parses `ovs-appctl dpctl/show`.
// Header line shape: "doca@ovs-doca:" then "  lookups: hit:N missed:N lost:N" and "  flows: N".
func parseDatapath(out string) DatapathInfo {
	var dp DatapathInfo
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, " "):
			dp.Name = strings.TrimSuffix(trimmed, ":")
		case strings.HasPrefix(trimmed, "lookups:"):
			dp.LookupsHit = grabUint(trimmed, "hit:")
			dp.LookupsMissed = grabUint(trimmed, "missed:")
			dp.LookupsLost = grabUint(trimmed, "lost:")
		case strings.HasPrefix(trimmed, "flows:"):
			dp.DPFlows = int(grabUint(trimmed, "flows:"))
		}
	}
	return dp
}

// parseCoverage extracts the `total: N` count for each wanted event from `coverage/show`.
func parseCoverage(out string, wanted []string) map[string]uint64 {
	want := toStrSet(wanted)
	res := map[string]uint64{}
	for _, e := range wanted {
		res[e] = 0 // default present-with-zero
	}
	for _, line := range strings.Split(out, "\n") {
		if m := reCoverage.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if want[m[1]] {
				if v, err := strconv.ParseUint(m[2], 10, 64); err == nil {
					res[m[1]] = v
				}
			}
		}
	}
	return res
}

// grabUint finds "<key><digits>" in s and returns the digits as uint64.
func grabUint(s, key string) uint64 {
	idx := strings.Index(s, key)
	if idx < 0 {
		return 0
	}
	rest := s[idx+len(key):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	v, _ := strconv.ParseUint(rest[:end], 10, 64)
	return v
}

func keysSorted(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortInts(out)
	return out
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func toSet(a []int) map[int]bool {
	s := make(map[int]bool, len(a))
	for _, v := range a {
		s[v] = true
	}
	return s
}

func toStrSet(a []string) map[string]bool {
	s := make(map[string]bool, len(a))
	for _, v := range a {
		s[v] = true
	}
	return s
}
```

> The `parseDatapath` header detection above is a starting point — adjust the case conditions to the exact `dpctl/show` shape in the fixture (the datapath header line is `doca@ovs-doca:` with no leading space; `lookups`/`flows` lines are indented). Verify against `testdata/dpctl-show.txt`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./components/ovs/collector/... -run TestParse -v`
Expected: PASS. If a parser assertion fails, fix the regex/conditions against the real fixture — do not change the assertion.

- [ ] **Step 5: Commit**

```bash
git add components/ovs/collector/parse.go components/ovs/collector/parse_test.go
git commit -m "feat(ovs): pure parsers for ovs-ofctl/appctl output with fixture tests"
```

---

## Task 6: Collector exec layer + availability gate

**Files:**
- Create: `components/ovs/collector/collector.go`

- [ ] **Step 1: Write `collector.go`**

```go
// <Apache header>
package collector

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/sirupsen/logrus"
)

type OVSCollector struct {
	name string
	spec *config.OVSSpec
}

func NewOVSCollector(spec *config.OVSSpec) *OVSCollector {
	return &OVSCollector{name: "OVSCollector", spec: spec}
}

func (c *OVSCollector) Name() string { return c.name }

// run executes a command and returns trimmed stdout, ignoring non-zero exit (caller checks output).
func run(ctx context.Context, name string, args ...string) string {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		logrus.WithField("component", "ovs").Debugf("cmd %s %v failed: %v", name, args, err)
	}
	return strings.TrimSpace(string(out))
}

func ovsGet(ctx context.Context, args ...string) string {
	return strings.Trim(run(ctx, "ovs-vsctl", args...), `"`)
}

// Collect satisfies common.Collector. On non-DOCA-OVS nodes it returns an
// OVSInfo with Available=false and no error.
func (c *OVSCollector) Collect(ctx context.Context) (common.Info, error) {
	info := &OVSInfo{
		Services:    map[string]string{},
		Packages:    map[string]string{},
		OtherConfig: map[string]string{},
		Coverage:    map[string]uint64{},
	}

	// Gate: ovs-vsctl on PATH + ovs-vswitchd active.
	if _, err := exec.LookPath("ovs-vsctl"); err != nil {
		info.Available = false
		info.SkipReason = "ovs-vsctl not found"
		return info, nil
	}
	for _, svc := range []string{"openvswitch-switch", "ovs-vswitchd", "ovsdb-server"} {
		info.Services[svc] = run(ctx, "systemctl", "is-active", svc)
	}
	if info.Services["ovs-vswitchd"] != "active" {
		info.Available = false
		info.SkipReason = "ovs-vswitchd not active"
		return info, nil
	}
	info.Available = true

	// Packages
	for _, pkg := range c.spec.RequiredPackages {
		info.Packages[pkg] = dpkgVersion(ctx, pkg)
	}

	// Versions + dpdk_initialized + other_config
	info.OVSVersion = ovsGet(ctx, "get", "Open_vSwitch", ".", "ovs_version")
	info.DPDKVersion = ovsGet(ctx, "get", "Open_vSwitch", ".", "dpdk_version")
	info.DPDKInitialized = ovsGet(ctx, "get", "Open_vSwitch", ".", "dpdk_initialized") == "true"
	for k := range c.spec.OtherConfig {
		info.OtherConfig[k] = ovsGet(ctx, "--if-exists", "get", "Open_vSwitch", ".", "other_config:"+k)
	}

	// Bridges
	for r := 0; r < c.spec.NumRails; r++ {
		br := fmt.Sprintf("%s%d", c.spec.BridgePrefix, r)
		info.Bridges = append(info.Bridges, c.collectBridge(ctx, br))
	}

	// Datapath / PMD / coverage (info + metrics only)
	info.Datapath = parseDatapath(run(ctx, "ovs-appctl", "dpctl/show"))
	info.Datapath.PMDs = parsePMDPerf(run(ctx, "ovs-appctl", "dpif-netdev/pmd-perf-show"))
	info.Coverage = parseCoverage(run(ctx, "ovs-appctl", "coverage/show"), c.spec.CoverageEvents)

	return info, nil
}

func (c *OVSCollector) collectBridge(ctx context.Context, br string) BridgeInfo {
	b := BridgeInfo{Name: br}
	// br-exists has no stdout, so detect membership via list-br instead.
	for _, x := range strings.Split(run(ctx, "ovs-vsctl", "list-br"), "\n") {
		if strings.TrimSpace(x) == br {
			b.Exists = true
			break
		}
	}
	if !b.Exists {
		return b
	}
	b.DatapathType = ovsGet(ctx, "get", "bridge", br, "datapath_type")
	b.FailMode = ovsGet(ctx, "get", "bridge", br, "fail_mode")
	ports := strings.Fields(run(ctx, "ovs-vsctl", "list-ports", br))
	b.Ports = len(ports)
	b.Flows = parseFlowCount(run(ctx, "ovs-ofctl", "dump-flows", br))
	b.GroupIDs = parseGroupIDs(run(ctx, "ovs-ofctl", "dump-groups", br))
	ofPorts := parseOFShowPorts(run(ctx, "ovs-ofctl", "show", br))
	refs := parseFlowPortRefs(run(ctx, "ovs-ofctl", "dump-flows", br))
	b.OrphanFlowRefs, b.OrphanPorts = diffPorts(ofPorts, refs)
	for _, p := range ports {
		pi := PortInfo{Name: p}
		pi.OFPort, _ = strconv.Atoi(ovsGet(ctx, "get", "interface", p, "ofport"))
		pi.AdminState = ovsGet(ctx, "get", "interface", p, "admin_state")
		pi.LinkState = ovsGet(ctx, "get", "interface", p, "link_state")
		if e := ovsGet(ctx, "get", "interface", p, "error"); e != "[]" {
			pi.Error = e
		}
		b.PortDetails = append(b.PortDetails, pi)
	}
	return b
}

// dpkgVersion returns the installed version, or "" if not installed (CurrentStatus != i).
func dpkgVersion(ctx context.Context, pkg string) string {
	out := run(ctx, "dpkg-query", "-W", "-f=${db:Status-Abbrev} ${Version}", pkg)
	fields := strings.Fields(out)
	if len(fields) < 2 || len(fields[0]) < 2 || fields[0][1] != 'i' {
		return ""
	}
	return fields[1]
}
```

> Confirm `fail_mode` is readable; if `ovs-vsctl get bridge <br> fail_mode` returns an error string (rather than e.g. `secure`), leave the parsed value as-is — it is informational, not part of any verdict.

- [ ] **Step 2: Add `parsePMDPerf` to `parse.go`**

```go
// parsePMDPerf parses `ovs-appctl dpif-netdev/pmd-perf-show` into per-core PMD stats.
// It is best-effort: the single-PMD summary form (no per-core breakdown) yields one entry
// with empty Core/NUMA. Adjust to the fixture shape captured in Task 4.
func parsePMDPerf(out string) []PMDInfo {
	var pmds []PMDInfo
	var cur *PMDInfo
	flush := func() {
		if cur != nil {
			pmds = append(pmds, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "pmd thread numa_id"):
			flush()
			cur = &PMDInfo{}
			cur.NUMA = grabField(t, "numa_id")
			cur.Core = grabField(t, "core_id")
		case cur != nil && strings.Contains(t, "idle iterations"):
			// counts appear on the Iterations/busy/idle lines; map as available
		}
	}
	flush()
	return pmds
}

// grabField returns the token following "<key> " up to whitespace or ','.
func grabField(s, key string) string {
	idx := strings.Index(s, key)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimLeft(s[idx+len(key):], " :")
	for i := 0; i < len(rest); i++ {
		if rest[i] == ' ' || rest[i] == ',' {
			return rest[:i]
		}
	}
	return rest
}
```

> PMD perf output shape varies by OVS build. Use the captured `testdata/pmd-perf-show.txt` to finalize `parsePMDPerf` and add a `TestParsePMDPerf` fixture test mirroring Task 5's style before relying on it. If the build only emits the aggregate `Iterations:/busy/idle` summary (as seen on zy3), populate a single `PMDInfo` with cycles from those lines and leave Core/NUMA empty.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./components/ovs/...`
Expected: success (after deleting the placeholder `br-exists` line noted above).

- [ ] **Step 4: Commit**

```bash
git add components/ovs/collector/
git commit -m "feat(ovs): collector exec layer with DOCA-OVS availability gate"
```

---

## Task 7: Checkers (TDD)

**Files:**
- Create: `components/ovs/checker/service_checker.go`
- Create: `components/ovs/checker/version_checker.go`
- Create: `components/ovs/checker/other_config_checker.go`
- Create: `components/ovs/checker/bridge_checker.go`
- Create: `components/ovs/checker/checker.go`
- Test: `components/ovs/checker/checker_test.go`

Level mapping (from spec): FAIL→`LevelCritical`; version-empty + orphan_ports→`LevelWarning`; nothing Fatal. Each checker returns `consts.StatusNormal`/`LevelInfo` when healthy.

- [ ] **Step 1: Write failing tests in `checker_test.go`**

```go
// <Apache header>
package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
)

func healthyInfo() *collector.OVSInfo {
	return &collector.OVSInfo{
		Available: true,
		Services: map[string]string{
			"openvswitch-switch": "active", "ovs-vswitchd": "active", "ovsdb-server": "active",
		},
		Packages: map[string]string{
			"doca-openvswitch-switch": "3.3.0040-1", "doca-openvswitch-common": "3.3.0040-1",
			"collectx-clxapi": "1.24.3", "libnvhws1": "26.01.9-1",
		},
		OVSVersion: "3.3.0040", DPDKVersion: "25.11.0", DPDKInitialized: true,
		OtherConfig: map[string]string{
			"doca-init": "true", "hw-offload": "true", "hw-offload-ct-size": "0",
			"max-idle": "300000", "doca-eswitch-max": "4",
		},
		Bridges: func() []collector.BridgeInfo {
			var bs []collector.BridgeInfo
			for r := 0; r < 8; r++ {
				bs = append(bs, collector.BridgeInfo{
					Name: "br-rail" + string(rune('0'+r)), Exists: true, DatapathType: "netdev",
					Ports: 5, Flows: 18, GroupIDs: []int{10, 20, 21, 22, 23, 30, 31, 32, 33},
				})
			}
			return bs
		}(),
	}
}

func TestServiceChecker_Healthy(t *testing.T) {
	c := &ServiceChecker{}
	r, err := c.Check(context.Background(), healthyInfo())
	assert.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, r.Status)
}

func TestServiceChecker_VswitchdDown(t *testing.T) {
	info := healthyInfo()
	info.Services["ovs-vswitchd"] = "inactive"
	r, _ := (&ServiceChecker{}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestOtherConfigChecker_Mismatch(t *testing.T) {
	info := healthyInfo()
	info.OtherConfig["hw-offload"] = "false"
	r, _ := (&OtherConfigChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestVersionChecker_PackageMissing(t *testing.T) {
	info := healthyInfo()
	info.Packages["libnvhws1"] = ""
	r, _ := (&VersionChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestVersionChecker_RuntimeEmptyIsWarning(t *testing.T) {
	info := healthyInfo()
	info.OVSVersion = ""
	r, _ := (&VersionChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelWarning, r.Level)
}

func TestBridgeChecker_MissingGroupID(t *testing.T) {
	info := healthyInfo()
	info.Bridges[0].GroupIDs = []int{10, 20, 21} // missing 22,23,30-33
	r, _ := (&BridgeChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestBridgeChecker_OrphanPortsIsWarning(t *testing.T) {
	info := healthyInfo()
	info.Bridges[0].OrphanPorts = []int{6}
	r, _ := (&BridgeChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelWarning, r.Level)
}

func TestBridgeChecker_Healthy(t *testing.T) {
	r, _ := (&BridgeChecker{spec: config.DefaultSpec()}).Check(context.Background(), healthyInfo())
	assert.Equal(t, consts.StatusNormal, r.Status)
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./components/ovs/checker/... -v`
Expected: FAIL — undefined checker types.

- [ ] **Step 3: Write `service_checker.go`**

```go
// <Apache header>
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/scitix/sichek/consts"
)

type ServiceChecker struct{}

func (c *ServiceChecker) Name() string { return config.ServiceCheckerName }

func (c *ServiceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for ServiceChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "OVS daemons active",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	for _, svc := range []string{"openvswitch-switch", "ovs-vswitchd", "ovsdb-server"} {
		if info.Services[svc] != "active" {
			r.Status = consts.StatusAbnormal
			r.Level = consts.LevelCritical
			r.ErrorName = "OVSServiceDown"
			r.Curr = "abnormal"
			r.Detail += fmt.Sprintf("service %s is %q (want active). ", svc, info.Services[svc])
			r.Suggestion = "Check `systemctl status " + svc + "` and DOCA-OVS deployment."
		}
	}
	return r, nil
}
```

- [ ] **Step 4: Write `version_checker.go`**

```go
// <Apache header>
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/scitix/sichek/consts"
)

type VersionChecker struct{ spec *config.OVSSpec }

func (c *VersionChecker) Name() string { return config.VersionCheckerName }

func (c *VersionChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for VersionChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "DOCA-OVS packages installed + runtime versions present",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	// Missing package => Critical.
	for _, pkg := range c.spec.RequiredPackages {
		if info.Packages[pkg] == "" {
			r.Status = consts.StatusAbnormal
			r.Level = consts.LevelCritical
			r.ErrorName = "OVSPackageMissing"
			r.Detail += fmt.Sprintf("package %s not installed. ", pkg)
		}
	}
	// Empty runtime version => Warning (only if not already Critical).
	if info.OVSVersion == "" || info.DPDKVersion == "" {
		r.Status = consts.StatusAbnormal
		if consts.LevelPriority[r.Level] < consts.LevelPriority[consts.LevelWarning] {
			r.Level = consts.LevelWarning
		}
		if r.ErrorName == "" {
			r.ErrorName = "OVSRuntimeVersionEmpty"
		}
		r.Detail += "OVS/DPDK runtime version empty (vswitchd may not be connected to DPDK). "
	}
	if r.Status == consts.StatusAbnormal {
		r.Curr = "abnormal"
		r.Suggestion = "Verify DOCA-OVS install (dpkg -l) and that ovs-vswitchd initialized DPDK."
	}
	return r, nil
}
```

- [ ] **Step 5: Write `other_config_checker.go`**

```go
// <Apache header>
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/scitix/sichek/consts"
)

type OtherConfigChecker struct{ spec *config.OVSSpec }

func (c *OtherConfigChecker) Name() string { return config.OtherConfigCheckerName }

func (c *OtherConfigChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for OtherConfigChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "Open_vSwitch other_config matches Step-8 spec",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	for k, want := range c.spec.OtherConfig {
		if got := info.OtherConfig[k]; got != want {
			r.Status = consts.StatusAbnormal
			r.Level = consts.LevelCritical
			r.ErrorName = "OVSOtherConfigMismatch"
			r.Detail += fmt.Sprintf("other_config:%s=%q want %q. ", k, got, want)
		}
	}
	if !info.DPDKInitialized {
		r.Status = consts.StatusAbnormal
		r.Level = consts.LevelCritical
		r.ErrorName = "OVSDpdkNotInitialized"
		r.Detail += "dpdk_initialized != true. "
	}
	if r.Status == consts.StatusAbnormal {
		r.Curr = "abnormal"
		r.Suggestion = "Re-apply rdma_env_pre Step 8 other_config and restart ovs-vswitchd."
	}
	return r, nil
}
```

- [ ] **Step 6: Write `bridge_checker.go`**

```go
// <Apache header>
package checker

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/scitix/sichek/consts"
)

type BridgeChecker struct{ spec *config.OVSSpec }

func (c *BridgeChecker) Name() string { return config.BridgeCheckerName }

func (c *BridgeChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for BridgeChecker")
	}
	r := &common.CheckerResult{
		Name: c.Name(), Description: "Per-rail bridge topology matches Step-10 spec",
		Status: consts.StatusNormal, Level: consts.LevelInfo, Curr: "OK",
	}
	raise := func(level string, errName, detail string) {
		r.Status = consts.StatusAbnormal
		if consts.LevelPriority[r.Level] < consts.LevelPriority[level] {
			r.Level = level
		}
		if r.ErrorName == "" || level == consts.LevelCritical {
			r.ErrorName = errName
		}
		r.Detail += detail
	}

	for _, b := range info.Bridges {
		if !b.Exists {
			raise(consts.LevelCritical, "OVSBridgeMissing", fmt.Sprintf("%s missing. ", b.Name))
			continue
		}
		if b.DatapathType != c.spec.DatapathType {
			raise(consts.LevelCritical, "OVSBridgeDatapath", fmt.Sprintf("%s datapath=%q want %q. ", b.Name, b.DatapathType, c.spec.DatapathType))
		}
		if b.Ports != c.spec.PortsPerBridge {
			raise(consts.LevelCritical, "OVSBridgePorts", fmt.Sprintf("%s ports=%d want %d. ", b.Name, b.Ports, c.spec.PortsPerBridge))
		}
		if b.Flows < c.spec.MinFlows {
			raise(consts.LevelCritical, "OVSBridgeFlows", fmt.Sprintf("%s flows=%d want >=%d. ", b.Name, b.Flows, c.spec.MinFlows))
		}
		if missing := missingInts(c.spec.ExpectedGroupIDs, b.GroupIDs); len(missing) > 0 {
			raise(consts.LevelCritical, "OVSBridgeGroupIDs", fmt.Sprintf("%s missing group_ids=%v. ", b.Name, missing))
		}
		if len(b.OrphanFlowRefs) > 0 {
			raise(consts.LevelCritical, "OVSOrphanFlowRefs", fmt.Sprintf("%s flows reference absent ports=%v. ", b.Name, b.OrphanFlowRefs))
		}
		if len(b.OrphanPorts) > 0 {
			raise(consts.LevelWarning, "OVSOrphanPorts", fmt.Sprintf("%s ports never referenced by a flow=%v. ", b.Name, b.OrphanPorts))
		}
	}
	if r.Status == consts.StatusAbnormal {
		r.Curr = "abnormal"
		r.Suggestion = "Re-run rdma_env_pre Step 10 bridge/group programming for the affected rails."
	}
	return r, nil
}

// missingInts returns elements of want not present in have.
func missingInts(want, have []int) []int {
	hset := make(map[int]bool, len(have))
	for _, v := range have {
		hset[v] = true
	}
	var missing []int
	for _, w := range want {
		if !hset[w] {
			missing = append(missing, w)
		}
	}
	return missing
}
```

- [ ] **Step 7: Write `checker.go`**

```go
// <Apache header>
package checker

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/config"
)

func NewCheckers(cfg *config.OVSUserConfig, spec *config.OVSSpec) ([]common.Checker, error) {
	all := []common.Checker{
		&ServiceChecker{},
		&VersionChecker{spec: spec},
		&OtherConfigChecker{spec: spec},
		&BridgeChecker{spec: spec},
	}
	ignored := map[string]bool{}
	if cfg != nil && cfg.OVS != nil {
		for _, v := range cfg.OVS.IgnoredCheckers {
			ignored[v] = true
		}
	}
	var active []common.Checker
	for _, chk := range all {
		if !ignored[chk.Name()] {
			active = append(active, chk)
		}
	}
	return active, nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./components/ovs/checker/... -v`
Expected: PASS (all 8 sub-tests). Note: the healthy-info helper builds bridge names with `string(rune('0'+r))` — fine for r<10.

- [ ] **Step 9: Commit**

```bash
git add components/ovs/checker/
git commit -m "feat(ovs): service/version/other_config/bridge checkers with tests"
```

---

## Task 8: Metrics package

**Files:**
- Create: `components/ovs/metrics/metrics.go`
- Test: `components/ovs/metrics/metrics_test.go`

Metrics mirror the rdma-doctor OVS schema under the `sichek_ovs` prefix. Use explicit `GaugeVecMetricExporter`s (one per label-shape), like `components/infiniband/metrics`.

- [ ] **Step 1: Write `metrics.go`**

```go
// <Apache header>
package metrics

import (
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	common "github.com/scitix/sichek/metrics"
)

const MetricPrefix = "sichek_ovs"

type OVSMetrics struct {
	expectedOtherConfig map[string]string // for other_config_ok comparison

	present    *common.GaugeVecMetricExporter // no extra labels
	serviceUp  *common.GaugeVecMetricExporter // {ovs_service}
	bridge     *common.GaugeVecMetricExporter // {bridge} -> flow/group/port/issue counts via metric name
	otherCfgOK *common.GaugeVecMetricExporter // {key}
	dpLookup   *common.GaugeVecMetricExporter // {datapath, result}
	dpFlows    *common.GaugeVecMetricExporter // {datapath}
	pmd        *common.GaugeVecMetricExporter // {core, numa}
	coverage   *common.GaugeVecMetricExporter // {event}
}

func NewOVSMetrics() *OVSMetrics {
	return &OVSMetrics{
		expectedOtherConfig: config.DefaultSpec().OtherConfig,
		present:             common.NewGaugeVecMetricExporter(MetricPrefix, nil),
		serviceUp:           common.NewGaugeVecMetricExporter(MetricPrefix, []string{"ovs_service"}),
		bridge:              common.NewGaugeVecMetricExporter(MetricPrefix, []string{"bridge"}),
		otherCfgOK:          common.NewGaugeVecMetricExporter(MetricPrefix, []string{"key"}),
		dpLookup:            common.NewGaugeVecMetricExporter(MetricPrefix, []string{"datapath", "result"}),
		dpFlows:             common.NewGaugeVecMetricExporter(MetricPrefix, []string{"datapath"}),
		pmd:                 common.NewGaugeVecMetricExporter(MetricPrefix, []string{"core", "numa"}),
		coverage:            common.NewGaugeVecMetricExporter(MetricPrefix, []string{"event"}),
	}
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (m *OVSMetrics) ExportMetrics(info *collector.OVSInfo) {
	if info == nil {
		return
	}
	m.present.SetMetric("present", nil, b2f(info.Available))
	if !info.Available {
		return
	}
	for svc, state := range info.Services {
		m.serviceUp.SetMetric("service_up", []string{svc}, b2f(state == "active"))
	}
	for _, b := range info.Bridges {
		m.bridge.SetMetric("bridge_flow_count", []string{b.Name}, float64(b.Flows))
		m.bridge.SetMetric("bridge_group_count", []string{b.Name}, float64(len(b.GroupIDs)))
		m.bridge.SetMetric("bridge_port_count", []string{b.Name}, float64(b.Ports))
		m.bridge.SetMetric("bridge_issue_count", []string{b.Name}, float64(len(b.OrphanFlowRefs)+len(b.OrphanPorts)))
	}
	for k, want := range m.expectedOtherConfig {
		m.otherCfgOK.SetMetric("other_config_ok", []string{k}, b2f(info.OtherConfig[k] == want))
	}
	m.dpFlows.SetMetric("datapath_flows", []string{info.Datapath.Name}, float64(info.Datapath.DPFlows))
	m.dpLookup.SetMetric("datapath_lookup", []string{info.Datapath.Name, "hit"}, float64(info.Datapath.LookupsHit))
	m.dpLookup.SetMetric("datapath_lookup", []string{info.Datapath.Name, "missed"}, float64(info.Datapath.LookupsMissed))
	m.dpLookup.SetMetric("datapath_lookup", []string{info.Datapath.Name, "lost"}, float64(info.Datapath.LookupsLost))
	for _, p := range info.Datapath.PMDs {
		m.pmd.SetMetric("pmd_busy_ratio", []string{p.Core, p.NUMA}, p.BusyRatio)
		m.pmd.SetMetric("pmd_idle_cycles", []string{p.Core, p.NUMA}, float64(p.IdleCycles))
		m.pmd.SetMetric("pmd_processing_cycles", []string{p.Core, p.NUMA}, float64(p.ProcessingCycles))
		m.pmd.SetMetric("pmd_rx_packets", []string{p.Core, p.NUMA}, float64(p.RxPackets))
	}
	for ev, total := range info.Coverage {
		m.coverage.SetMetric("coverage_total", []string{ev}, float64(total))
	}
}
```

- [ ] **Step 2: Write `metrics_test.go`**

```go
// <Apache header>
package metrics

import (
	"testing"

	"github.com/scitix/sichek/components/ovs/collector"
)

func TestExportMetrics_DoesNotPanic(t *testing.T) {
	m := NewOVSMetrics()
	m.ExportMetrics(&collector.OVSInfo{Available: false})
	m.ExportMetrics(&collector.OVSInfo{
		Available: true,
		Services:  map[string]string{"ovs-vswitchd": "active"},
		Bridges:   []collector.BridgeInfo{{Name: "br-rail0", Ports: 5, Flows: 18, GroupIDs: []int{10}}},
		Datapath:  collector.DatapathInfo{Name: "doca@ovs-doca", LookupsHit: 5},
		Coverage:  map[string]uint64{"flow_offload_200ms_latency": 1},
	})
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./components/ovs/metrics/... -v`
Expected: PASS. (Prometheus `MustRegister` panics on duplicate registration across tests in the same process — `GaugeVecMetricExporter` registers lazily per name; one test instance is fine.)

- [ ] **Step 4: Commit**

```bash
git add components/ovs/metrics/
git commit -m "feat(ovs): prometheus metrics mirroring rdma-doctor ovs schema"
```

---

## Task 9: Component wiring (`ovs.go`)

**Files:**
- Create: `components/ovs/ovs.go`

This implements `common.Component`, mirroring `components/transceiver/transceiver.go` exactly (lifecycle methods are identical except types). Below is the full file.

- [ ] **Step 1: Write `ovs.go`**

```go
// <Apache header>
package ovs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ovs/checker"
	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	ovsmetrics "github.com/scitix/sichek/components/ovs/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.OVSUserConfig
	cfgMutex      sync.Mutex
	collector     *collector.OVSCollector
	checkers      []common.Checker
	metrics       *ovsmetrics.OVSMetrics

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	ovsComponent     *component
	ovsComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string, ignoredCheckers []string) (common.Component, error) {
	var err error
	ovsComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component ovs: %v", r)
			}
		}()
		ovsComponent, err = newComponent(cfgFile, specFile, ignoredCheckers)
	})
	return ovsComponent, err
}

func newComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.OVSUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.OVS == nil {
		logrus.WithField("component", "ovs").Warnf("get user config failed or ovs config is nil, using default config")
		cfg.OVS = &config.OVSConfig{
			QueryInterval: common.Duration{Duration: 60 * time.Second},
			CacheSize:     5,
			EnableMetrics: true,
		}
	}
	if len(ignoredCheckers) > 0 {
		cfg.OVS.IgnoredCheckers = ignoredCheckers
	}

	spec, err := config.LoadSpec(specFile)
	if err != nil {
		logrus.WithField("component", "ovs").Warnf("failed to load spec %s: %v", specFile, err)
	}

	checkers, err := checker.NewCheckers(cfg, spec)
	if err != nil {
		return nil, err
	}

	cacheSize := cfg.OVS.CacheSize
	if cacheSize == 0 {
		cacheSize = 5
	}

	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameOVS,
		collector:     collector.NewOVSCollector(spec),
		checkers:      checkers,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cacheSize),
		cacheInfo:     make([]common.Info, cacheSize),
		cacheSize:     cacheSize,
		metrics:       ovsmetrics.NewOVSMetrics(),
	}
	comp.service = common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	return comp, nil
}

func (c *component) Name() string { return c.componentName }

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "ovs").Errorf("failed to collect ovs info: %v", err)
		return nil, err
	}
	ovsInfo, ok := info.(*collector.OVSInfo)
	if !ok {
		return nil, fmt.Errorf("wrong ovs collector info type")
	}

	if c.cfg.OVS != nil && c.cfg.OVS.EnableMetrics {
		c.metrics.ExportMetrics(ovsInfo)
	}

	var result *common.Result
	if !ovsInfo.Available {
		// Graceful no-op on non-DOCA-OVS nodes: one info checker, no issues.
		result = &common.Result{
			Item: c.componentName, Status: consts.StatusNormal, Level: consts.LevelInfo,
			Time: time.Now(),
			Checkers: []*common.CheckerResult{{
				Name: "ovs_present", Description: "DOCA-OVS availability",
				Status: consts.StatusNormal, Level: consts.LevelInfo,
				Curr: "skipped", Detail: ovsInfo.SkipReason,
			}},
		}
	} else {
		result = common.Check(ctx, c.componentName, ovsInfo, c.checkers)
	}

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = ovsInfo
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	return result, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheBuffer, nil
}

func (c *component) LastResult() (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfo, nil
}

func (c *component) LastInfo() (common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	if c.currIndex == 0 {
		return c.cacheInfo[c.cacheSize-1], nil
	}
	return c.cacheInfo[c.currIndex-1], nil
}

func (c *component) Start() <-chan *common.Result { return c.service.Start() }
func (c *component) Stop() error                  { return c.service.Stop() }
func (c *component) Status() bool                 { return c.service.Status() }
func (c *component) GetTimeout() time.Duration    { return c.cfg.GetQueryInterval().Duration }

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	cp, ok := cfg.(*config.OVSUserConfig)
	if !ok {
		c.cfgMutex.Unlock()
		return fmt.Errorf("update wrong config type for ovs")
	}
	c.cfg = cp
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := !(result.Status == consts.StatusAbnormal && consts.LevelPriority[result.Level] > consts.LevelPriority[consts.LevelInfo])
	utils.PrintTitle("OVS", "-")
	ovsInfo, ok := info.(*collector.OVSInfo)
	if !ok || ovsInfo == nil {
		fmt.Println("No OVS info available")
		return checkAllPassed
	}
	if !ovsInfo.Available {
		fmt.Printf("OVS not active on this node: %s\n", ovsInfo.SkipReason)
		return checkAllPassed
	}
	fmt.Printf("OVS %s / DPDK %s  dpdk_initialized=%v\n", ovsInfo.OVSVersion, ovsInfo.DPDKVersion, ovsInfo.DPDKInitialized)
	fmt.Printf("%-12s %-9s %-7s %-7s %s\n", "bridge", "datapath", "ports", "flows", "groups")
	for _, b := range ovsInfo.Bridges {
		fmt.Printf("%-12s %-9s %-7d %-7d %d\n", b.Name, b.DatapathType, b.Ports, b.Flows, len(b.GroupIDs))
	}
	if result != nil {
		for _, res := range result.Checkers {
			if res.Status != consts.StatusNormal && res.Level != consts.LevelInfo {
				fmt.Printf("\tEvent: %s%s%s -> %s\n", consts.LevelColor(res.Level), res.ErrorName, consts.Reset, res.Detail)
			}
		}
	}
	fmt.Println()
	return checkAllPassed
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./components/ovs/...`
Expected: success. (If `common.LoadUserConfig` or `utils.PrintTitle` signatures differ, match the transceiver usage you read.)

- [ ] **Step 3: Commit**

```bash
git add components/ovs/ovs.go
git commit -m "feat(ovs): component wiring implementing common.Component"
```

---

## Task 10: CLI subcommand + factory registration

**Files:**
- Create: `cmd/command/component/ovs.go`
- Modify: `cmd/command/component/all.go`
- Modify: `cmd/command/command.go`

- [ ] **Step 1: Write `cmd/command/component/ovs.go`**

Mirror `cmd/command/component/transceiver.go`:

```go
// <Apache header>
package component

import (
	"context"
	"strings"

	"github.com/scitix/sichek/cmd/command/spec"
	"github.com/scitix/sichek/components/ovs"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewOVSCmd() *cobra.Command {
	var (
		cfgFile            string
		specFile           string
		ignoredCheckersStr string
		verbose            bool
	)
	ovsCmd := &cobra.Command{
		Use:     "ovs",
		Short:   "Perform OVS (DOCA Open vSwitch) HealthCheck",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				logrus.SetLevel(logrus.DebugLevel)
				defer cancel()
			}
			resolvedCfgFile, err := spec.EnsureCfgFile(cfgFile)
			if err != nil {
				logrus.WithField("daemon", "ovs").Errorf("failed to load cfgFile: %v", err)
			}
			resolvedSpecFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("daemon", "ovs").Errorf("failed to load specFile: %v", err)
			}
			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}
			component, err := ovs.NewComponent(resolvedCfgFile, resolvedSpecFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", "ovs").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}
	ovsCmd.Flags().StringVarP(&cfgFile, "cfg", "c", "", "Path to the user config file")
	ovsCmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to the ovs specification file")
	ovsCmd.Flags().StringVarP(&ignoredCheckersStr, "ignored-checkers", "i", "", "Ignored checkers")
	ovsCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	return ovsCmd
}
```

> Caution re EnsureSpecFile: per repo memory, `EnsureSpecFile`/`EnsureCfgFile` write into `/var/sichek/config/...` and may overwrite prod canonical config/spec. This matches every other subcommand's behavior, so it is consistent — but during field testing set `SICHEK_CONFIG_DIR=/tmp/...` to avoid clobbering prod (see field-regression skill).

- [ ] **Step 2: Add factory case in `all.go`**

Add the import (with the other component imports, ~line 38):

```go
	"github.com/scitix/sichek/components/ovs"
```

Add the switch case in `NewComponent` (after the transceiver case, ~line 195):

```go
	case consts.ComponentNameOVS:
		return ovs.NewComponent(cfgFile, specFile, ignoredCheckers)
```

- [ ] **Step 3: Register subcommand in `command.go`**

After line 80 (`rootCmd.AddCommand(component.NewTransceiverCmd())`), add:

```go
	rootCmd.AddCommand(component.NewOVSCmd())
```

- [ ] **Step 4: Verify build + CLI**

Run: `go build ./... && go run ./cmd ovs --help`
Expected: build succeeds; `ovs` help prints. On a non-OVS dev box, `go run ./cmd ovs` exits 0 with "OVS not active on this node".

- [ ] **Step 5: Commit**

```bash
git add cmd/command/component/ovs.go cmd/command/component/all.go cmd/command/command.go
git commit -m "feat(ovs): cli subcommand + factory registration"
```

---

## Task 11: Annotation + snapshot wiring (`service/info.go`)

**Files:**
- Modify: `service/info.go`

Without this, OVS issues are silently dropped from the K8s annotation and `snapshot.Issues` (the `Components["ovs"]` map is populated automatically by the daemon and needs no change).

- [ ] **Step 1: Add the `OVS` field to `nodeAnnotation`**

In the `nodeAnnotation` struct (line ~29-42), add:

```go
	OVS         map[string][]*annotation `json:"ovs"`
```

- [ ] **Step 2: Add case to `getAnnotationsByItem`**

In the switch (line ~95-118), add before the closing brace:

```go
	case consts.ComponentNameOVS:
		return a.OVS, nil
```

- [ ] **Step 3: Add case to `setAnnotationsByItem`**

In the switch (line ~126-149), add:

```go
	case consts.ComponentNameOVS:
		a.OVS = annotations
```

- [ ] **Step 4: Verify build**

Run: `go build ./service/...`
Expected: success.

- [ ] **Step 5: Add a regression test for the annotation round-trip**

Append to the existing `service/info_test.go` (or create it if absent):

```go
func TestNodeAnnotation_OVSRoundTrip(t *testing.T) {
	a := &nodeAnnotation{}
	anns := map[string][]*annotation{"br-rail0": {{}}}
	require.NoError(t, a.setAnnotationsByItem(consts.ComponentNameOVS, anns))
	got, err := a.getAnnotationsByItem(consts.ComponentNameOVS)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
```

(Import `consts`, `testify/assert`, `testify/require` if not present.)

- [ ] **Step 6: Run + commit**

Run: `go test ./service/... -run TestNodeAnnotation_OVSRoundTrip -v`
Expected: PASS.

```bash
git add service/info.go service/info_test.go
git commit -m "feat(ovs): wire ovs into node annotation + snapshot issues schema"
```

---

## Task 12: Full build + vet + integration check

**Files:** none (verification only)

- [ ] **Step 1: Full build and vet**

Run: `go build ./... && go vet ./components/ovs/... ./cmd/... ./service/...`
Expected: no errors.

- [ ] **Step 2: Full test run**

Run: `go test ./components/ovs/... ./service/...`
Expected: PASS.

- [ ] **Step 3: Confirm `ovs` is in the daemon component set**

Run: `grep -n "ComponentNameOVS" consts/consts.go cmd/command/component/all.go service/info.go`
Expected: name constant + DefaultComponents + factory case + 3 info.go references all present.

- [ ] **Step 4: Commit (if any vet fixes were needed)**

```bash
git add -A && git commit -m "chore(ovs): vet fixes + final wiring"
```

---

## Task 13: Field regression on zy3

**Files:** none (validation)

Use the `sichek-field-regression` skill. Key points: build linux/amd64, scp to `/tmp/sichek-test`, run on zy3. Do NOT pass `NODE_NAME=zy` (zy3 spec resolution — see repo memory). Set `SICHEK_CONFIG_DIR=/tmp/sichek-cfg` to avoid clobbering prod config/spec.

- [ ] **Step 1: Build + deploy**

```bash
make   # or: GOOS=linux GOARCH=amd64 go build -o build/bin/sichek ./cmd
scp build/bin/sichek zy3:/tmp/sichek-test
```

- [ ] **Step 2: One-shot CLI on the positive node**

Run: `ssh zy3 'SICHEK_CONFIG_DIR=/tmp/sichek-cfg /tmp/sichek-test ovs -v'`
Expected: prints bridge table for br-rail0..7, all checkers PASS (healthy node), exit 0.

- [ ] **Step 2b: Confirm a non-OVS node no-ops**

Run on any non-DOCA-OVS node: `/tmp/sichek-test ovs`
Expected: "OVS not active on this node", exit 0, no issues.

- [ ] **Step 3: Daemon path — snapshot + metrics**

Start the daemon (or the exporter path) per the field-regression skill, then:

```bash
# snapshot has both Components.ovs and (if any issue) Issues.ovs
ssh zy3 'cat /var/sichek/snapshot.json | python3 -c "import sys,json;d=json.load(sys.stdin);print(\"components.ovs present:\", \"ovs\" in d[\"components\"]); print(json.dumps(d[\"components\"].get(\"ovs\",{}),indent=2)[:400])"'
# prometheus
ssh zy3 'curl -s http://127.0.0.1:19092/metrics | grep "^sichek_ovs_" | head -40'
```

Expected: `components.ovs present: True`; `sichek_ovs_*` metrics present (service_up, bridge_*, datapath_*, present=1).

- [ ] **Step 4: Compare against rdma-doctor (sanity)**

Run: `ssh zy3 'curl -sk https://127.0.0.1:9188/metrics | grep "^rdma_doctor_ovs_bridge_port_count"'`
Compare bridge/port/group counts with sichek's `sichek_ovs_bridge_*` — they should agree on port/group counts (flow counts may differ by counting method, expected).

- [ ] **Step 5: Produce the regression report** (Markdown, keyed to acceptance criteria), per the skill.

---

## Self-Review

**Spec coverage:**
- Gating/availability → Task 6 (`Collect` gate) + Task 9 (no-op result). ✔
- OVSInfo → snapshot.Components → Task 3 + Task 9 (`LastInfo`) + daemon (no change needed). ✔
- 4 checkers (Service/Version/OtherConfig/Bridge) → Task 7. ✔
- Topology/Datapath = info+metrics only, no verdict → no datapath checker created (Task 7 omits it); metrics in Task 8. ✔
- Spec baselines (default_ovs_spec.yaml) → Task 2. ✔
- Prometheus `sichek_ovs_*` mirroring rdma-doctor → Task 8. ✔
- Level mapping (Critical / Warning / no Fatal) → Task 7 tests assert exact levels. ✔
- Wiring: consts+DefaultComponents (Task 1), factory+CLI (Task 10), annotation schema (Task 11). ✔
- Tests + field regression → Tasks 5,7,8,11,12,13. ✔

**Placeholder scan:** One spot is legitimately fixture-dependent: `parsePMDPerf` (Task 6, Step 2) must be finalized against the captured `testdata/pmd-perf-show.txt`, because the PMD-perf output shape varies by OVS build (zy3 emits the aggregate `Iterations:/busy/idle` summary, not a per-core breakdown). The task includes explicit instructions and requires adding a fixture test before relying on it — it is not a silent TODO. All other code blocks are complete and correct as written.

**Type consistency:** `OVSInfo`/`BridgeInfo`/`PMDInfo` field names used in checker, metrics, and ovs.go match Task 3. Checker names (`ovs_service`/`ovs_version`/`ovs_other_config`/`ovs_bridge`) defined once in `config/config.go` (Task 2) and referenced everywhere. `config.OVSSpec` field names consistent across spec.go, collector, checkers, metrics.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-30-ovs-component.md`. Two execution options:

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
