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
	"os"
	"sort"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/lldp/collector"
	"github.com/scitix/sichek/consts"
)

// RailCheckerName identifies the LLDP rail-consistency checker.
const RailCheckerName = "lldp-rail-consistency"

// RailChecker validates that every RDMA card of the same model (board_id)
// uplinks to the same switch port position ("rail"). A GPU server occupies one
// rack slot N, so on every leaf switch of a given fabric it should land on the
// same switch-side port number N. Compute / storage / mgmt HCAs carry distinct
// board_ids, so grouping by board_id judges each fabric independently: a
// wiring/rail error in one fabric never masks or false-flags another.
type RailChecker struct {
	name     string
	hostname string
}

// NewRailChecker builds a RailChecker resolving the local hostname (used to
// tell a real switch uplink apart from a looped-back self neighbor).
func NewRailChecker() (common.Checker, error) {
	hostname, _ := os.Hostname()
	return newRailCheckerWithHost(hostname), nil
}

func newRailCheckerWithHost(hostname string) *RailChecker {
	return &RailChecker{name: RailCheckerName, hostname: hostname}
}

func (c *RailChecker) Name() string { return c.name }

func (c *RailChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.LldpInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type, expected *collector.LldpInfo")
	}

	result := common.CheckerResult{
		Name:        RailCheckerName,
		Description: "Check that same-model (board_id) RDMA cards uplink to a consistent switch port position (rail)",
		Device:      consts.ComponentNameLLDP,
		Spec:        "consistent",
		Level:       consts.LevelWarning,
		ErrorName:   "LLDPRailInconsistent",
		Suggestion:  "Same-model cards land on different switch port tails; check cabling/rail order for the flagged fabric",
		Status:      consts.StatusNormal,
		Curr:        "consistent",
	}

	if !info.LldpdAvailable {
		result.Detail = "lldpd not available; rail check skipped"
		result.Suggestion = ""
		return &result, nil
	}

	// Group switch-uplink interfaces by board_id (one group per NIC model /
	// fabric), recording the switch-side rail key for each.
	type member struct {
		local   string
		chassis string
		portID  string
		rail    string
	}
	groups := make(map[string][]member)
	for _, iface := range info.Interfaces {
		if iface.Local.BoardID == "" {
			continue
		}
		if !isRailUplink(iface.Neighbor, c.hostname) {
			continue
		}
		rail, ok := railKey(iface.Neighbor.Port.ID)
		if !ok {
			continue
		}
		bid := iface.Local.BoardID
		groups[bid] = append(groups[bid], member{
			local:   iface.Local.Name,
			chassis: iface.Neighbor.Chassis.Name,
			portID:  iface.Neighbor.Port.ID,
			rail:    rail,
		})
	}

	// A fabric is inconsistent when its members show more than one distinct
	// rail key. Judge each board_id group independently.
	var badBoards []string
	var details []string
	for _, bid := range sortedKeys(groups) {
		members := groups[bid]
		rails := make(map[string]bool)
		for _, m := range members {
			rails[m.rail] = true
		}
		if len(rails) <= 1 {
			continue
		}
		badBoards = append(badBoards, bid)
		var rows []string
		for _, m := range members {
			rows = append(rows, fmt.Sprintf("%s->%s(%s)=rail%s", m.local, m.chassis, m.portID, m.rail))
		}
		details = append(details, fmt.Sprintf("board_id %s: %s", bid, strings.Join(rows, ", ")))
	}

	if len(badBoards) > 0 {
		result.Status = consts.StatusAbnormal
		result.Curr = "inconsistent"
		result.Detail = strings.Join(details, "; ")
		return &result, nil
	}

	result.Detail = "all fabrics have consistent switch port rails"
	result.Suggestion = ""
	return &result, nil
}

// isRailUplink reports whether a neighbor is a real rail-switch uplink that
// should participate in rail grouping. A switch names its port either by
// "ifname" (H3C/Arista) or "local" (SONiC/Spectrum-X), so both are accepted.
// Excluded are self/loopback links (chassis is this very host, e.g. OVS VF
// representors) and host-to-host links (peer identifies its port by MAC).
func isRailUplink(n collector.Neighbor, hostname string) bool {
	if n.Port.IDType == "mac" {
		return false
	}
	if hostname != "" && n.Chassis.Name == hostname {
		return false
	}
	return n.Port.ID != ""
}

// railKey derives a rail identity from a switch-side LLDP port id — the slot
// position a card occupies on its leaf switch. Same-model cards on the same
// rack slot must land on the same key across every leaf of their fabric.
//
// Two switch port-naming families are handled:
//
//	H3C/Arista slash form -> trailing number of the last "/"-segment, after
//	dropping a ":lane" suffix and a trailing breakout letter:
//	  FourHundredGigE1/0/98 -> "98"
//	  TwoHundredGigE1/0/1:1  -> "1"
//	  Ethernet19/1          -> "1"
//	  ethernet12a           -> "12"
//
//	SONiC breakout form (no slash) -> cage+subport kept whole, so 54p1 and
//	54p2 are distinct rails:
//	  Ethernet54p2 -> "54p2"
//	  Ethernet7p0  -> "7p0"
//
// Returns ok=false when the id carries no digits to key on.
func railKey(portID string) (string, bool) {
	s := strings.TrimSpace(portID)
	if s == "" {
		return "", false
	}

	if strings.ContainsRune(s, '/') {
		// Slash form (H3C/Arista): drop a ":lane" suffix, take the last
		// "/"-segment, key on its trailing digit run.
		if i := strings.IndexByte(s, ':'); i >= 0 {
			s = s[:i]
		}
		if i := strings.LastIndexByte(s, '/'); i >= 0 {
			s = s[i+1:]
		}
		digits := trailingDigits(s)
		if digits == "" {
			return "", false
		}
		return digits, true
	}

	// SONiC breakout form: strip a leading non-digit prefix ("Ethernet") and
	// keep the rest (cage + optional pN subport) verbatim.
	i := 0
	for i < len(s) && !isASCIIDigit(s[i]) {
		i++
	}
	if i == len(s) {
		return "", false
	}
	return s[i:], true
}

func trailingDigits(s string) string {
	end := len(s)
	start := end
	for start > 0 && isASCIIDigit(s[start-1]) {
		start--
	}
	return s[start:end]
}

func isASCIIDigit(b byte) bool { return b >= '0' && b <= '9' }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
