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

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoErrorf(t, err, "read fixture %s", name)
	return b
}

func TestParseLldpctlJSON_Clnet36(t *testing.T) {
	got, err := ParseLldpctlJSON(loadFixture(t, "clnet36.json"))
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Iface with a single-string mgmt-ip (eth0)
	eth0List, ok := got["eth0"]
	require.True(t, ok, "eth0 missing")
	require.Len(t, eth0List, 1)
	eth0 := eth0List[0]
	assert.Equal(t, "CL-RLF233", eth0.Chassis.Name)
	assert.Equal(t, "mac", eth0.Chassis.IDType)
	assert.Equal(t, "90:74:2e:ed:72:c0", eth0.Chassis.ID)
	assert.Equal(t, []string{"26.9.135.254"}, eth0.Chassis.MgmtIP)
	assert.ElementsMatch(t, []string{"Bridge", "Router"}, eth0.Chassis.Capability)
	assert.Equal(t, "FourHundredGigE1/0/98", eth0.Port.ID)
	assert.Equal(t, "ifname", eth0.Port.IDType)
	assert.Equal(t, 9216, eth0.Port.MFS) // mfs as quoted string
	assert.Equal(t, 52, eth0.VlanID)
	assert.True(t, eth0.VlanPVID)
	assert.Equal(t, int64(55*86400+25*60+58), eth0.AgeSeconds)

	// Iface with no mgmt-ip but auto-neg present (ens14f0np0)
	ensList, ok := got["ens14f0np0"]
	require.True(t, ok)
	require.Len(t, ensList, 1)
	ens := ensList[0]
	assert.Empty(t, ens.Chassis.MgmtIP)
	assert.Contains(t, ens.Port.AutoNegCurrent, "25GbaseSR")
}

func TestParseLldpctlJSON_Dracog24MultiMgmtIP(t *testing.T) {
	got, err := ParseLldpctlJSON(loadFixture(t, "dracog24.json"))
	require.NoError(t, err)
	require.Len(t, got, 1)

	eth1List, ok := got["eth1"]
	require.True(t, ok)
	require.Len(t, eth1List, 1)
	eth1 := eth1List[0]
	// mgmt-ip is a list here (v4 + v6); both should be preserved.
	assert.Equal(t,
		[]string{"33.239.145.151", "fdbd:dc41:199:4901::21ef:9197"},
		eth1.Chassis.MgmtIP)

	// capability list mixes enabled/disabled; only the enabled ones should
	// appear in our flattened slice.
	assert.ElementsMatch(t, []string{"Bridge", "Router"}, eth1.Chassis.Capability)

	// No VLAN block at all in this fixture.
	assert.Equal(t, 0, eth1.VlanID)
	assert.False(t, eth1.VlanPVID)
}

func TestParseLldpctlJSON_Bjg45VlanWithValue(t *testing.T) {
	got, err := ParseLldpctlJSON(loadFixture(t, "bjg45.json"))
	require.NoError(t, err)
	require.Len(t, got, 1)

	ensList, ok := got["ens11f0"]
	require.True(t, ok)
	require.Len(t, ensList, 1)
	ens := ensList[0]
	assert.Equal(t, 1, ens.VlanID)
	assert.True(t, ens.VlanPVID)
	assert.Equal(t, "VLAN1", ens.VlanName)
	assert.Equal(t, "139", ens.Port.AggregationID)
}

func TestParseLldpctlJSON_Zy1RailFabric(t *testing.T) {
	got, err := ParseLldpctlJSON(loadFixture(t, "zy1.json"))
	require.NoError(t, err)
	require.Len(t, got, 52, "zy1 has 8 rails x4 ports + storage + mgmt + VF-rep loopbacks")

	// Compute rail uplink: real switch port advertised as ifname.
	p0List, ok := got["eth_r0_p0"]
	require.True(t, ok)
	require.Len(t, p0List, 1)
	p0 := p0List[0]
	assert.Equal(t, "KL-R0LF01", p0.Chassis.Name)
	assert.Equal(t, "ifname", p0.Port.IDType)
	assert.Equal(t, "TwoHundredGigE1/0/1:1", p0.Port.ID)

	// Storage leaf uplink.
	require.Len(t, got["s_eth0"], 1)
	assert.Equal(t, "KL-SLF01", got["s_eth0"][0].Chassis.Name)

	// OVS VF representor: chassis is this host itself (a self loopback, not a
	// switch uplink). The display layer filters these out by hostname.
	vfList, ok := got["eth_vf_rep_r0"]
	require.True(t, ok)
	require.Len(t, vfList, 1)
	vf := vfList[0]
	assert.Equal(t, "hydra-gpu-214-171-47-1", vf.Chassis.Name)
}

func TestParseLldpctlJSON_Lmg86HostNeighbor(t *testing.T) {
	got, err := ParseLldpctlJSON(loadFixture(t, "lmg86.json"))
	require.NoError(t, err)
	require.Len(t, got, 6)

	// eth0 neighbor is another host, not a switch: it identifies its port by
	// MAC (id_type "mac") rather than an ifname. The display layer uses this
	// to exclude host-to-host links from the switch-uplink table.
	eth0List, ok := got["eth0"]
	require.True(t, ok)
	require.Len(t, eth0List, 1)
	eth0 := eth0List[0]
	assert.Equal(t, "mac", eth0.Port.IDType)

	// eth1 is a real switch uplink (ifname port on a Lambda leaf).
	eth1List, ok := got["eth1"]
	require.True(t, ok)
	require.Len(t, eth1List, 1)
	eth1 := eth1List[0]
	assert.Equal(t, "ifname", eth1.Port.IDType)
	assert.Equal(t, "ethernet12a", eth1.Port.ID)
}

func TestParseLldpctlJSON_Empty(t *testing.T) {
	got, err := ParseLldpctlJSON(loadFixture(t, "empty.json"))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseLldpctlJSON_EmptyInput(t *testing.T) {
	got, err := ParseLldpctlJSON([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseLldpctlJSON_InvalidJSON(t *testing.T) {
	_, err := ParseLldpctlJSON([]byte("{not json"))
	require.Error(t, err)
}

func TestParseLldpctlJSON_InterfaceAsObject(t *testing.T) {
	// Some lldpctl builds emit "interface" as a single object (no array)
	// when there is exactly one entry. The parser must handle both shapes.
	raw := []byte(`{
		"lldp": {
			"interface": {
				"eth9": {
					"via": "LLDP",
					"age": "1 day, 00:00:05",
					"chassis": { "sw1": { "id": {"type":"mac","value":"aa:bb:cc:dd:ee:ff"} } },
					"port": { "id": {"type":"ifname","value":"Gi1/0/1"} }
				}
			}
		}
	}`)
	got, err := ParseLldpctlJSON(raw)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Len(t, got["eth9"], 1)
	assert.Equal(t, "sw1", got["eth9"][0].Chassis.Name)
	assert.Equal(t, "Gi1/0/1", got["eth9"][0].Port.ID)
	assert.Equal(t, int64(86400+5), got["eth9"][0].AgeSeconds)
}

func TestParseLldpctlJSON_Clnet16PreservesAllNeighbors(t *testing.T) {
	// clnet16 reports several neighbors on eth0: a real switch uplink
	// (CL-RLF237, port advertised as ifname) plus the host's own LLDP frames
	// looped back from VLAN subinterfaces (chassis == this hostname, port by
	// MAC). The parser must preserve every neighbor in encounter order rather
	// than collapsing to the last one seen.
	got, err := ParseLldpctlJSON(loadFixture(t, "clnet16.json"))
	require.NoError(t, err)

	eth0 := got["eth0"]
	require.Len(t, eth0, 3, "eth0 should keep the switch + 2 loopbacks")
	// Switch uplink is first and must not be overwritten by the loopbacks.
	assert.Equal(t, "CL-RLF237", eth0[0].Chassis.Name)
	assert.Equal(t, "ifname", eth0[0].Port.IDType)
	assert.Equal(t, "FourHundredGigE1/0/126", eth0[0].Port.ID)
	// The remaining entries are the host's self loopbacks (port id by MAC).
	assert.Equal(t, "changliu-g86-016", eth0[1].Chassis.Name)
	assert.Equal(t, "mac", eth0[1].Port.IDType)

	require.Len(t, got["ens14f0np0"], 1)
	require.Len(t, got["vlanonly0"], 1)
}

func TestSelectUplinkNeighbor(t *testing.T) {
	got, err := ParseLldpctlJSON(loadFixture(t, "clnet16.json"))
	require.NoError(t, err)
	const hostname = "changliu-g86-016"

	tests := []struct {
		name      string
		iface     string
		wantName  string
		wantIDTyp string
	}{
		{
			name:      "switch preferred over looped-back self neighbors",
			iface:     "eth0",
			wantName:  "CL-RLF237",
			wantIDTyp: "ifname",
		},
		{
			name:      "single switch neighbor kept as-is",
			iface:     "ens14f0np0",
			wantName:  "CL-A054A",
			wantIDTyp: "ifname",
		},
		{
			name:      "only self loopbacks falls back to first neighbor",
			iface:     "vlanonly0",
			wantName:  hostname,
			wantIDTyp: "mac",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := selectUplinkNeighbor(got[tt.iface], hostname)
			assert.Equal(t, tt.wantName, n.Chassis.Name)
			assert.Equal(t, tt.wantIDTyp, n.Port.IDType)
		})
	}

	// Empty input is safe.
	assert.Equal(t, Neighbor{}, selectUplinkNeighbor(nil, hostname))
}

func TestIsSwitchNeighbor(t *testing.T) {
	tests := []struct {
		name     string
		n        Neighbor
		hostname string
		want     bool
	}{
		{
			name:     "ifname port on a remote switch is an uplink",
			n:        Neighbor{Chassis: Chassis{Name: "CL-RLF237"}, Port: Port{IDType: "ifname"}},
			hostname: "node-a",
			want:     true,
		},
		{
			name:     "mac port is not an uplink",
			n:        Neighbor{Chassis: Chassis{Name: "CL-RLF237"}, Port: Port{IDType: "mac"}},
			hostname: "node-a",
			want:     false,
		},
		{
			name:     "ifname but chassis is our own hostname (self loopback)",
			n:        Neighbor{Chassis: Chassis{Name: "node-a"}, Port: Port{IDType: "ifname"}},
			hostname: "node-a",
			want:     false,
		},
		{
			name:     "empty hostname cannot match self, so ifname is treated as uplink",
			n:        Neighbor{Chassis: Chassis{Name: "node-a"}, Port: Port{IDType: "ifname"}},
			hostname: "",
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSwitchNeighbor(tt.n, tt.hostname))
		})
	}
}

func TestParseAge(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"00:00:05", 5},
		{"00:01:00", 60},
		{"1 day, 00:00:00", 86400},
		{"55 days, 00:25:58", 55*86400 + 25*60 + 58},
		{"84 days, 03:03:34", 84*86400 + 3*3600 + 3*60 + 34},
	}
	for _, c := range cases {
		got := parseAge(c.in)
		assert.Equal(t, c.want, got, "input=%q", c.in)
	}
}
