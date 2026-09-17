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

	"github.com/scitix/sichek/components/lldp/collector"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRailKey(t *testing.T) {
	cases := []struct {
		in     string
		wantK  string
		wantOK bool
	}{
		// H3C/Arista slash form: last segment number.
		{"FourHundredGigE1/0/98", "98", true},
		{"FourHundredGigE1/0/126", "126", true},
		{"Twenty-FiveGigE1/0/16", "16", true},
		{"TwoHundredGigE1/0/1:1", "1", true}, // strip ":lane"
		{"25GE1/0/39", "39", true},
		{"25ge0/1", "1", true},
		{"HundredGigE1/0/11", "11", true},
		{"Ethernet19/1", "1", true},   // Arista mgmt (ifname)
		{"1/0/7", "7", true},          // bare slash form
		// Slashless breakout forms keep the subport/lane designator whole, per
		// the SONiC rail-key ruling: 12a and 12b are distinct physical ports.
		{"ethernet12a", "12a", true},
		{"ethernet12b", "12b", true},
		{"Eth1/25", "25", true},
		// SONiC breakout form: keep cage+subport whole (54p1 != 54p2).
		{"Ethernet54p2", "54p2", true},
		{"Ethernet54p1", "54p1", true},
		{"Ethernet7p0", "7p0", true},
		{"", "", false},       // empty
		{"mgmt0", "0", true},  // trailing digit
		{"lo", "", false},     // no digits at all
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := railKey(c.in)
			assert.Equal(t, c.wantOK, ok)
			if c.wantOK {
				assert.Equal(t, c.wantK, got)
			}
		})
	}
}

// switchIface builds an IfaceInfo for a real switch uplink advertised by
// ifname (H3C/Arista style).
func switchIface(local, boardID, chassis, portID string) collector.IfaceInfo {
	return switchIfaceT(local, boardID, chassis, portID, "ifname")
}

// switchIfaceT is switchIface with an explicit port.id_type, so tests can
// exercise SONiC switches that advertise their port as "local".
func switchIfaceT(local, boardID, chassis, portID, idType string) collector.IfaceInfo {
	return collector.IfaceInfo{
		Local: collector.LocalIface{Name: local, BoardID: boardID},
		Neighbor: collector.Neighbor{
			Chassis: collector.Chassis{Name: chassis},
			Port:    collector.Port{ID: portID, IDType: idType},
		},
	}
}

func TestRailChecker_Check(t *testing.T) {
	const hostname = "gpu-host-1"

	t.Run("three fabrics each consistent -> normal", func(t *testing.T) {
		// compute (board A), storage (board B), mgmt (board C); each fabric's
		// switch-side port tails are internally consistent.
		info := &collector.LldpInfo{
			LldpdAvailable: true,
			Interfaces: []collector.IfaceInfo{
				switchIface("ib0", "boardA", "compute-sw-1", "FourHundredGigE1/0/7"),
				switchIface("ib1", "boardA", "compute-sw-2", "FourHundredGigE1/0/7"),
				switchIface("ib2", "boardA", "compute-sw-3", "FourHundredGigE1/0/7"),
				switchIface("eth4", "boardB", "storage-sw-1", "TwoHundredGigE1/0/12"),
				switchIface("eth5", "boardB", "storage-sw-2", "TwoHundredGigE1/0/12"),
				switchIface("eth0", "boardC", "mgmt-sw-1", "Twenty-FiveGigE1/0/3"),
			},
		}
		c := newRailCheckerWithHost(hostname)
		res, err := c.Check(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusNormal, res.Status)
	})

	t.Run("compute fabric tail mismatch -> warning, other fabrics unaffected", func(t *testing.T) {
		info := &collector.LldpInfo{
			LldpdAvailable: true,
			Interfaces: []collector.IfaceInfo{
				switchIface("ib0", "boardA", "compute-sw-1", "FourHundredGigE1/0/7"),
				switchIface("ib1", "boardA", "compute-sw-2", "FourHundredGigE1/0/7"),
				switchIface("ib2", "boardA", "compute-sw-3", "FourHundredGigE1/0/91"), // odd one out
				// storage fabric stays consistent; must not be flagged
				switchIface("eth4", "boardB", "storage-sw-1", "TwoHundredGigE1/0/12"),
				switchIface("eth5", "boardB", "storage-sw-2", "TwoHundredGigE1/0/12"),
			},
		}
		c := newRailCheckerWithHost(hostname)
		res, err := c.Check(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusAbnormal, res.Status)
		assert.Equal(t, consts.LevelWarning, res.Level)
		// Detail names the offending board and both tail values, but not the
		// healthy storage board.
		assert.Contains(t, res.Detail, "boardA")
		assert.Contains(t, res.Detail, "7")
		assert.Contains(t, res.Detail, "91")
		assert.NotContains(t, res.Detail, "boardB")
	})

	t.Run("self-loopback and host neighbors excluded from grouping", func(t *testing.T) {
		info := &collector.LldpInfo{
			LldpdAvailable: true,
			Interfaces: []collector.IfaceInfo{
				switchIface("ib0", "boardA", "compute-sw-1", "FourHundredGigE1/0/7"),
				switchIface("ib1", "boardA", "compute-sw-2", "FourHundredGigE1/0/7"),
				// self VF-rep loopback: chassis == hostname. Tail 99 would break
				// the group if it were (wrongly) counted.
				{
					Local: collector.LocalIface{Name: "eth_r0", BoardID: "boardA"},
					Neighbor: collector.Neighbor{
						Chassis: collector.Chassis{Name: hostname},
						Port:    collector.Port{ID: "FourHundredGigE1/0/99", IDType: "ifname"},
					},
				},
				// host-to-host neighbor: port identified by MAC.
				{
					Local: collector.LocalIface{Name: "eth9", BoardID: "boardA"},
					Neighbor: collector.Neighbor{
						Chassis: collector.Chassis{Name: "peer.byted.org"},
						Port:    collector.Port{ID: "aa:bb:cc:dd:ee:ff", IDType: "mac"},
					},
				},
			},
		}
		c := newRailCheckerWithHost(hostname)
		res, err := c.Check(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusNormal, res.Status)
	})

	t.Run("SONiC local-type uplinks are judged, consistent -> normal", func(t *testing.T) {
		// Spectrum-X leaf switches (SONiC) advertise the port as "local", not
		// "ifname". These MUST participate; skipping them is the bug this case
		// guards. Same cage+subport on every leaf -> consistent.
		info := &collector.LldpInfo{
			LldpdAvailable: true,
			Interfaces: []collector.IfaceInfo{
				switchIfaceT("eth0", "boardX", "GY-SLF03", "Ethernet54p2", "local"),
				switchIfaceT("eth1", "boardX", "GY-SLF04", "Ethernet54p2", "local"),
			},
		}
		c := newRailCheckerWithHost(hostname)
		res, err := c.Check(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusNormal, res.Status)
	})

	t.Run("SONiC local-type subport mismatch -> warning", func(t *testing.T) {
		// 54p1 and 54p2 are different rails: must flag.
		info := &collector.LldpInfo{
			LldpdAvailable: true,
			Interfaces: []collector.IfaceInfo{
				switchIfaceT("eth0", "boardX", "GY-SLF03", "Ethernet54p2", "local"),
				switchIfaceT("eth1", "boardX", "GY-SLF04", "Ethernet54p1", "local"),
			},
		}
		c := newRailCheckerWithHost(hostname)
		res, err := c.Check(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusAbnormal, res.Status)
		assert.Equal(t, consts.LevelWarning, res.Level)
		assert.Contains(t, res.Detail, "54p2")
		assert.Contains(t, res.Detail, "54p1")
	})

	t.Run("ifaces without board_id are skipped", func(t *testing.T) {
		info := &collector.LldpInfo{
			LldpdAvailable: true,
			Interfaces: []collector.IfaceInfo{
				switchIface("ib0", "", "compute-sw-1", "FourHundredGigE1/0/7"),
				switchIface("ib1", "", "compute-sw-2", "FourHundredGigE1/0/91"),
			},
		}
		c := newRailCheckerWithHost(hostname)
		res, err := c.Check(context.Background(), info)
		require.NoError(t, err)
		// No board_id -> no groups -> nothing to judge -> normal.
		assert.Equal(t, consts.StatusNormal, res.Status)
	})

	t.Run("lldpd unavailable -> normal (nothing to judge)", func(t *testing.T) {
		info := &collector.LldpInfo{LldpdAvailable: false, Reason: "lldpctl not found"}
		c := newRailCheckerWithHost(hostname)
		res, err := c.Check(context.Background(), info)
		require.NoError(t, err)
		assert.Equal(t, consts.StatusNormal, res.Status)
	})

	t.Run("wrong data type -> error", func(t *testing.T) {
		c := newRailCheckerWithHost(hostname)
		_, err := c.Check(context.Background(), "not-lldp-info")
		assert.Error(t, err)
	})
}
