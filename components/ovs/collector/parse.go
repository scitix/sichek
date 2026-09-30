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
	"regexp"
	"strconv"
	"strings"
)

var (
	reGroupID  = regexp.MustCompile(`group_id=(\d+)`)
	reOFShow   = regexp.MustCompile(`^\s+(\d+)\(`)
	rePortRef  = regexp.MustCompile(`(?:in_port=|output:)(\d+)`)
	reCoverage = regexp.MustCompile(`^(\S+)\s.*total:\s*(\d+)`)
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
// Header line shape: "doca@ovs-doca:" (no leading whitespace) then indented
// "  lookups: hit:N missed:N lost:N" and "  flows: N".
func parseDatapath(out string) DatapathInfo {
	var dp DatapathInfo
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(line, " ") && trimmed != "":
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

// parsePMDPerf parses `ovs-appctl dpif-netdev/pmd-perf-show` into per-core PMD
// stats. The DOCA-OVS output on this hardware is PER-CORE: one block per
// "pmd thread numa_id <N> core_id <C>:" line. For each block we capture:
//   - Core/NUMA from the header line
//   - IdleCycles  = the "- idle iterations:" count (best-available proxy for
//     idle cycles; this build reports iteration counts, not raw cycles)
//   - ProcessingCycles = the "- busy iterations:" count (busy-iteration proxy)
//   - RxPackets   = the "Rx packets:" count
//   - BusyRatio   = busy iterations / total Iterations (0 when Iterations==0)
func parsePMDPerf(out string) []PMDInfo {
	var pmds []PMDInfo
	var cur *PMDInfo
	var iterations uint64
	flush := func() {
		if cur != nil {
			if iterations > 0 {
				cur.BusyRatio = float64(cur.ProcessingCycles) / float64(iterations)
			}
			pmds = append(pmds, *cur)
			cur = nil
			iterations = 0
		}
	}
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "pmd thread numa_id"):
			flush()
			cur = &PMDInfo{
				NUMA: grabField(t, "numa_id"),
				Core: grabField(t, "core_id"),
			}
		case cur == nil:
			// skip lines outside any PMD block
		case strings.HasPrefix(t, "Iterations:"):
			iterations = grabFirstUint(t[len("Iterations:"):])
		case strings.HasPrefix(t, "- idle iterations:"):
			cur.IdleCycles = grabFirstUint(t[len("- idle iterations:"):])
		case strings.HasPrefix(t, "- busy iterations:"):
			cur.ProcessingCycles = grabFirstUint(t[len("- busy iterations:"):])
		case strings.HasPrefix(t, "Rx packets:"):
			cur.RxPackets = grabFirstUint(t[len("Rx packets:"):])
		}
	}
	flush()
	return pmds
}

// grabField returns the token following "<key>" (skipping leading spaces/colons)
// up to the next whitespace or ','.
func grabField(s, key string) string {
	_, rest, found := strings.Cut(s, key)
	if !found {
		return ""
	}
	rest = strings.TrimLeft(rest, " :")
	for i := 0; i < len(rest); i++ {
		if rest[i] == ' ' || rest[i] == ',' || rest[i] == ':' {
			return rest[:i]
		}
	}
	return rest
}

// parseOVSStatMap extracts a uint64 value for key from an OVS map string such as
// `{rx_bytes=5598796, tx_bytes=12, rx_errors=0}`. Returns 0 if the key is absent.
func parseOVSStatMap(s, key string) uint64 {
	_, rest, found := strings.Cut(s, key+"=")
	if !found {
		return 0
	}
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

// grabFirstUint returns the first run of digits found in s as a uint64.
func grabFirstUint(s string) uint64 {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			start = i
			break
		}
	}
	if start < 0 {
		return 0
	}
	end := start
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	v, _ := strconv.ParseUint(s[start:end], 10, 64)
	return v
}

// grabUint finds "<key><digits>" in s and returns the digits as uint64.
func grabUint(s, key string) uint64 {
	_, rest, found := strings.Cut(s, key)
	if !found {
		return 0
	}
	rest = strings.TrimLeft(rest, " ")
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
