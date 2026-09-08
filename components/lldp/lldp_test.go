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
package lldp

import (
	"testing"

	"github.com/scitix/sichek/components/lldp/collector"

	"github.com/stretchr/testify/assert"
)

func TestIsSwitchUplink(t *testing.T) {
	const hostname = "hydra-gpu-214-171-47-1"
	cases := []struct {
		name  string
		iface collector.IfaceInfo
		want  bool
	}{
		{
			name: "real switch uplink (ifname port, remote chassis)",
			iface: collector.IfaceInfo{
				Neighbor: collector.Neighbor{
					Chassis: collector.Chassis{Name: "KL-R0LF01"},
					Port:    collector.Port{ID: "TwoHundredGigE1/0/1:1", IDType: "ifname"},
				},
			},
			want: true,
		},
		{
			name: "self VF-rep loopback (chassis is this host)",
			iface: collector.IfaceInfo{
				Neighbor: collector.Neighbor{
					Chassis: collector.Chassis{Name: hostname},
					Port:    collector.Port{ID: "eth_r0", IDType: "ifname"},
				},
			},
			want: false,
		},
		{
			name: "host-to-host neighbor (port identified by MAC)",
			iface: collector.IfaceInfo{
				Neighbor: collector.Neighbor{
					Chassis: collector.Chassis{Name: "p22-128-067.byted.org"},
					Port:    collector.Port{ID: "96:23:5f:0c:b0:61", IDType: "mac"},
				},
			},
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, collector.IsSwitchNeighbor(c.iface.Neighbor, hostname))
		})
	}
}

func TestFmtAge(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "-"},
		{-1, "-"},
		{5 * 60, "5m"},
		{3*3600 + 4*60, "3h4m"},
		{182*86400 + 0*3600, "182d0h"},
		{55*86400 + 25*60 + 58, "55d0h"},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, fmtAge(c.in), "fmtAge(%d)", c.in)
	}
}

func TestVlanCell(t *testing.T) {
	assert.Equal(t, "-", vlanCell(collector.Neighbor{VlanID: 0}))
	assert.Equal(t, "52", vlanCell(collector.Neighbor{VlanID: 52}))
	assert.Equal(t, "52*", vlanCell(collector.Neighbor{VlanID: 52, VlanPVID: true}))
}

func TestClip(t *testing.T) {
	assert.Equal(t, "eth0", clip("eth0", 20))
	assert.Equal(t, "CL-RLF233", clip("CL-RLF233", 9)) // exactly at limit: unchanged
	assert.Equal(t, "CL-RLF2+", clip("CL-RLF233", 8))  // over limit: truncated with '+'
	assert.Len(t, clip("0123456789abcdef", 8), 8)
}
