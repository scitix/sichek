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

func TestParseLaneFlags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []int
	}{
		{name: "ethtool per-lane list", input: "[ No, No, Yes, Yes, No, No, No, No ]", want: []int{0, 0, 1, 1, 0, 0, 0, 0}},
		{name: "ethtool single value", input: "Yes", want: []int{1}},
		{name: "mlxlink per-lane", input: "0,1", want: []int{0, 1}},
		{name: "ethtool none means no lane flagged, lane count unknown here", input: "None", want: nil},
		{name: "mlxlink not available", input: "N/A", want: nil},
		{name: "garbage", input: "[ No, maybe ]", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseLaneFlags(tt.input))
		})
	}
}

// Captured from eth0 (CX7, 200G over 2 lanes) on hydra-g34-214-171-42-107: the
// 8-lane module flags Rx LOS on channels 3/4, which are dark (-40 dBm) because
// the link only uses channels 1/2. Trimming to the link width must drop them.
func TestEthtoolLaneFlagsDarkLanes(t *testing.T) {
	var m ModuleInfo
	m.parseEthtoolModule(readFixture(t, "ethtool_m_cmis_dark_lanes.txt"))
	assert.Equal(t, []int{0, 0, 1, 1, 0, 0, 0, 0}, m.RxLOS)
	// "None" expands to one clear flag per module lane.
	assert.Equal(t, make([]int, 8), m.TxLOS)
	assert.Equal(t, make([]int, 8), m.RxLOL)
	assert.Equal(t, make([]int, 8), m.TxLOL)
	assert.Equal(t, make([]int, 8), m.TxFault)

	m.LinkLanes = 2
	trimToLinkLanes(&m)
	assert.Equal(t, []int{0, 0}, m.RxLOS)
	assert.Equal(t, []int{0, 0}, m.TxFault)
}

func TestEthtoolLaneFlagsQSFP28(t *testing.T) {
	var m ModuleInfo
	m.parseEthtoolModule(readFixture(t, "ethtool_m_qsfp28.txt"))
	n := len(m.RxPower)
	assert.Greater(t, n, 0)
	assert.Equal(t, make([]int, n), m.RxLOS)
	assert.Equal(t, make([]int, n), m.TxFault)
}

// A module without DOM lanes has nothing to size "None" against; leave the
// flags empty rather than invent lanes.
func TestEthtoolLaneFlagsNoLanes(t *testing.T) {
	var m ModuleInfo
	m.parseEthtoolModule("\tRx loss of signal : None\n\tTx fault : None\n")
	assert.Nil(t, m.RxLOS)
	assert.Nil(t, m.TxFault)
}

// Captured from roce_r0 (0000:17:00.0, CX8 plane PF) via mlxlink -m.
func TestMLXLinkLaneFlags(t *testing.T) {
	var m ModuleInfo
	m.parseMLXLink(readFixture(t, "mlxlink_m_cx8_plane.txt"))
	assert.Equal(t, []int{0, 0}, m.RxLOS)
	assert.Equal(t, []int{0, 0}, m.TxLOS)
	assert.Equal(t, []int{0, 0}, m.RxLOL)
	assert.Equal(t, []int{0, 0}, m.TxLOL)
	assert.Equal(t, []int{0, 0}, m.TxFault)

	m = ModuleInfo{}
	m.parseMLXLink("Rx LOS [per lane]                  : 0,1\nRx CDR LOL [per lane]              : 1,0\n")
	assert.Equal(t, []int{0, 1}, m.RxLOS)
	assert.Equal(t, []int{1, 0}, m.RxLOL)
	// "Tx Adaptive EQ Fault" must not be read as Tx Fault.
	m = ModuleInfo{}
	m.parseMLXLink("Tx Adaptive EQ Fault [per lane]    : 1,1\n")
	assert.Nil(t, m.TxFault)
}

func TestSplitSharedModuleLanesFlags(t *testing.T) {
	mk := func(bdf string) indexedModule {
		return indexedModule{
			iface: InterfaceEntry{PcieBDF: bdf},
			module: ModuleInfo{
				Present: true, CollectTool: "ethtool", SerialNumber: "SN1",
				RxPower: make([]float64, 8),
				RxLOS:   []int{0, 0, 0, 1, 0, 0, 0, 0},
				TxFault: make([]int, 8),
			},
		}
	}
	items := []indexedModule{mk("0000:19:00.1"), mk("0000:19:00.0"), mk("0000:19:00.2"), mk("0000:19:00.3")}
	splitSharedModuleLanes(items)
	byBDF := map[string]ModuleInfo{}
	for _, it := range items {
		byBDF[it.iface.PcieBDF] = it.module
	}
	assert.Equal(t, []int{0, 0}, byBDF["0000:19:00.0"].RxLOS)
	assert.Equal(t, []int{0, 1}, byBDF["0000:19:00.1"].RxLOS, "lane 4 belongs to plane 1")
	assert.Equal(t, []int{0, 0}, byBDF["0000:19:00.3"].TxFault)
}
