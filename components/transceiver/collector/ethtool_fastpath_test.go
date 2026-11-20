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

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(data)
}

// The CMIS (OSFP 8x) fixture was captured from a B300 + CX8 multi-plane node.
// One `ethtool -m` call on any plane PF returns all 8 lanes of the cage.
func TestParseEthtoolModuleCMIS(t *testing.T) {
	m := &ModuleInfo{}
	m.parseEthtoolModule(readFixture(t, "ethtool_m_cmis_osfp.txt"))

	assert.Equal(t, "zsine", m.Vendor)
	assert.Equal(t, "TRJ5J0PPNFDMR000", m.PartNumber)
	assert.Equal(t, "XWKG265Q00005097", m.SerialNumber)
	assert.Contains(t, m.ModuleType, "OSFP")
	assert.InDelta(t, 42.7, m.Temperature, 0.5)
	assert.InDelta(t, 3.30, m.Voltage, 0.05)

	require.Len(t, m.TxPower, 8, "CMIS 'Transmit avg optical power' lanes")
	require.Len(t, m.RxPower, 8, "CMIS 'Rcvr signal avg optical power' lanes")
	require.Len(t, m.BiasCurrent, 8)
	assert.InDelta(t, 3.01, m.RxPower[0], 1e-9)
	assert.InDelta(t, 2.58, m.TxPower[0], 1e-9)

	// Per-lane alarm *flags* ("Laser tx power high alarm (Channel 1): Off") must
	// not be mistaken for power readings.
	for i, v := range m.TxPower {
		assert.NotZero(t, v, "tx lane %d polluted by an alarm flag", i+1)
	}

	// CMIS names the tx threshold "Laser output power ..."; rx keeps "Laser rx power ...".
	assert.InDelta(t, 5.00, m.TxPowerHighAlarm, 1e-9)
	assert.InDelta(t, -3.90, m.TxPowerLowAlarm, 1e-9)
	assert.InDelta(t, 5.00, m.RxPowerHighAlarm, 1e-9)
	assert.InDelta(t, -6.90, m.RxPowerLowAlarm, 1e-9)
	assert.InDelta(t, 75.0, m.TempHighAlarm, 1e-9)
	assert.InDelta(t, 2.97, m.VoltageLowAlarm, 1e-9)
}

func TestHasUsableDOM(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "decoded CMIS", output: readFixture(t, "ethtool_m_cmis_osfp.txt"), want: true},
		{name: "decoded SFP", output: ethtoolSampleOutput, want: true},
		// ethtool exits 0 but prints only a raw dump when it cannot decode the
		// module's memory map; that must fall back to mlxlink.
		{name: "raw hex dump only", output: "Offset\t\tValues\n------\t\t------\n0x0000:\t\t19 00 06 00 00 00 00 00\n", want: false},
		{name: "empty", output: "", want: false},
		// mlx5 often serves only the SFP identity page (A0h) to ethtool and not
		// the diagnostics page (A2h): vendor fields decode but there are no
		// readings. mlxlink still has the DOM, so this must fall back too.
		{name: "SFP identity page without DOM", output: readFixture(t, "ethtool_m_sfp_no_dom.txt"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ModuleInfo{}
			m.parseEthtoolModule(tt.output)
			assert.Equal(t, tt.want, m.hasUsableDOM())
		})
	}
}

func TestParseEthtoolStats(t *testing.T) {
	m := &ModuleInfo{}
	m.parseEthtoolStats(readFixture(t, "ethtool_S_mlx5.txt"))

	// The same underlying PHY counters mlxlink -c reports.
	assert.Equal(t, uint64(0), m.EffectivePhysicalErrors) // rx_pcs_symbol_err_phy
	assert.Equal(t, uint64(0), m.LinkDownCounter)         // link_down_events_phy
	assert.Equal(t, uint64(0), m.LinkErrorRecoveryCounter)
	assert.Equal(t, uint64(75977848), m.RawPhysicalErrors) // rx_corrected_bits_phy

	// Legacy link-error keys stay populated for check_link_errors.
	require.NotNil(t, m.LinkErrors)
}

func TestParseMLXLinkRecommendationPlain(t *testing.T) {
	m := &ModuleInfo{}
	m.parseMLXLink(readFixture(t, "mlxlink_plain.txt"))
	assert.Equal(t, "No issue was observed", m.Recommendation)
	// Plain mlxlink (no -m) carries no DOM, so it must not clobber ethtool data.
	assert.Empty(t, m.RxPower)
	assert.Empty(t, m.Vendor)
}

func TestSplitSharedModuleLanes(t *testing.T) {
	lanes := func(base float64) []float64 {
		out := make([]float64, 8)
		for i := range out {
			out[i] = base + float64(i)
		}
		return out
	}
	mk := func(bdf, sn string) indexedModule {
		return indexedModule{
			iface: InterfaceEntry{PcieBDF: bdf},
			module: ModuleInfo{
				Present: true, CollectTool: "ethtool", SerialNumber: sn,
				RxPower: lanes(10), TxPower: lanes(20), BiasCurrent: lanes(30),
			},
		}
	}

	t.Run("4 plane PFs sharing one OSFP get 2 lanes each in function order", func(t *testing.T) {
		items := []indexedModule{
			mk("0000:19:00.2", "SN1"), mk("0000:19:00.0", "SN1"),
			mk("0000:19:00.3", "SN1"), mk("0000:19:00.1", "SN1"),
			mk("0000:4c:00.0", "SN2"),
		}
		splitSharedModuleLanes(items)

		byBDF := map[string]ModuleInfo{}
		for _, it := range items {
			byBDF[it.iface.PcieBDF] = it.module
		}
		// Verified on hardware: plane function N owns lanes 2N+1, 2N+2.
		assert.Equal(t, []float64{10, 11}, byBDF["0000:19:00.0"].RxPower)
		assert.Equal(t, []float64{12, 13}, byBDF["0000:19:00.1"].RxPower)
		assert.Equal(t, []float64{24, 25}, byBDF["0000:19:00.2"].TxPower)
		assert.Equal(t, []float64{36, 37}, byBDF["0000:19:00.3"].BiasCurrent)
		// A module not shared with another PF keeps all of its lanes.
		assert.Len(t, byBDF["0000:4c:00.0"].RxPower, 8)
	})

	t.Run("modules without a serial number are never grouped", func(t *testing.T) {
		items := []indexedModule{mk("0000:19:00.0", ""), mk("0000:19:00.1", "")}
		splitSharedModuleLanes(items)
		assert.Len(t, items[0].module.RxPower, 8)
		assert.Len(t, items[1].module.RxPower, 8)
	})

	t.Run("lane count not divisible by group size is left untouched", func(t *testing.T) {
		items := []indexedModule{mk("0000:19:00.0", "SN1"), mk("0000:19:00.1", "SN1"), mk("0000:19:00.2", "SN1")}
		splitSharedModuleLanes(items)
		for _, it := range items {
			assert.Len(t, it.module.RxPower, 8)
		}
	})

	t.Run("mlxlink-collected rows are already per-plane and not split", func(t *testing.T) {
		a, b := mk("0000:19:00.0", "SN1"), mk("0000:19:00.1", "SN1")
		a.module.CollectTool, b.module.CollectTool = "mlxlink", "mlxlink"
		items := []indexedModule{a, b}
		splitSharedModuleLanes(items)
		assert.Len(t, items[0].module.RxPower, 8)
	})
}

func TestParseEthtoolLinkLanes(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{name: "200G over 2 lanes", output: "Settings for eth0:\n\tSpeed: 200000Mb/s\n\tLanes: 2\n\tDuplex: Full\n", want: 2},
		{name: "link down has no lanes line", output: "Settings for eth0:\n\tSpeed: Unknown!\n", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseEthtoolLinkLanes(tt.output))
		})
	}
}

// A 4-lane module running a 2-lane link: ethtool -m reports all module lanes,
// the unused ones dark (bias 0, Tx ~-16 dBm). mlxlink only ever showed the
// active lanes, so the ethtool path must trim to the link width or the dark
// lanes trip the power and bias checkers.
func TestTrimToLinkLanes(t *testing.T) {
	m := ModuleInfo{
		LinkLanes:   2,
		TxPower:     []float64{4.41, 3.99, -16.74, -16.27},
		RxPower:     []float64{2.68, 2.86, -40, -40},
		BiasCurrent: []float64{6.06, 6.06, 12.2, 12.2},
	}
	trimToLinkLanes(&m)
	assert.Equal(t, []float64{4.41, 3.99}, m.TxPower)
	assert.Equal(t, []float64{2.68, 2.86}, m.RxPower)
	assert.Equal(t, []float64{6.06, 6.06}, m.BiasCurrent)

	unknown := ModuleInfo{RxPower: []float64{1, 2, 3, 4}}
	trimToLinkLanes(&unknown)
	assert.Len(t, unknown.RxPower, 4, "unknown link width keeps every lane")
}
