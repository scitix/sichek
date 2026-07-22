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
	ports := parseOFShowPorts(show)  // ofport numbers present on the bridge
	refs := parseFlowPortRefs(flows) // in_port=/output: numbers referenced by flows
	orphanRefs, orphanPorts := diffPorts(ports, refs)
	assert.Empty(t, orphanRefs, "healthy node should have no orphan flow refs")
	_ = orphanPorts
}

func TestParseDatapathLookups(t *testing.T) {
	out := readFixture(t, "dpctl-show.txt")
	dp := parseDatapath(out)
	assert.Equal(t, "doca@ovs-doca", dp.Name)
	assert.Greater(t, dp.LookupsHit, uint64(0))
	assert.Greater(t, dp.LookupsMissed, uint64(0))
	assert.Greater(t, dp.DPFlows, 0)
}

func TestParseCoverage(t *testing.T) {
	out := readFixture(t, "coverage-show.txt")
	cov := parseCoverage(out, []string{"flow_offload_200ms_latency", "doca_datapath_drop_upcall_error"})
	_, ok := cov["doca_datapath_drop_upcall_error"]
	assert.True(t, ok)
	assert.Equal(t, uint64(1), cov["doca_datapath_drop_upcall_error"])
	assert.Equal(t, uint64(1), cov["flow_offload_200ms_latency"])
}

func TestParseOVSStatMap(t *testing.T) {
	const stats = "{rx_bytes=5598796, tx_bytes=12, rx_errors=0}"
	tests := []struct {
		name string
		in   string
		key  string
		want uint64
	}{
		{"rx_bytes", stats, "rx_bytes", 5598796},
		{"tx_bytes", stats, "tx_bytes", 12},
		{"rx_errors zero", stats, "rx_errors", 0},
		{"missing key", stats, "tx_errors", 0},
		{"empty input", "", "rx_bytes", 0},
		{"no braces", "rx_bytes=42", "rx_bytes", 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseOVSStatMap(tt.in, tt.key))
		})
	}
}

func TestParsePMDPerf(t *testing.T) {
	out := readFixture(t, "pmd-perf-show.txt")
	pmds := parsePMDPerf(out)
	require.Len(t, pmds, 4)

	var found *PMDInfo
	for i := range pmds {
		if pmds[i].Core == "21" && pmds[i].NUMA == "0" {
			found = &pmds[i]
			break
		}
	}
	require.NotNil(t, found, "expected a PMD with core_id=21 numa_id=0")
	assert.Equal(t, uint64(35839), found.RxPackets)
	assert.Greater(t, found.BusyRatio, float64(0))
	// busy iterations / idle iterations populated as proxies
	assert.Equal(t, uint64(12670), found.ProcessingCycles)
	assert.Equal(t, uint64(998623801194), found.IdleCycles)
}
