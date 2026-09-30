package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const mlxlinkSampleOutput = `
Module Info
-----------
Temperature [C]                    : 54 [-5..75]
Voltage [mV]                       : 3261.2 [2970..3630]
Bias Current [mA]                  : 8.380,8.380,8.380,8.380 [2..15]
Rx Power Current [dBm]             : 1.886,1.989,2.281,1.976 [-10.41..6]
Tx Power Current [dBm]             : 1.562,1.427,1.559,1.761 [-8.508..5]
Identifier                         : OSFP
Vendor Name                        : CLT
Vendor Part Number                 : T-F4GS-BV0
Vendor Serial Number               : CWJH05025502589
`

// mlxlinkSampleWithRecommendation mirrors real `mlxlink -d <dev> -m` output:
// the default "Troubleshooting Info" section (carrying Recommendation) prints
// alongside the "-m" Module Info section. The recommendation here is wrapped in
// ANSI red color codes, as mlxlink emits on a terminal ("爆红").
const mlxlinkSampleWithRecommendation = "\n" +
	"Module Info\n" +
	"-----------\n" +
	"Temperature [C]                    : 54 [-5..75]\n" +
	"Identifier                         : OSFP\n" +
	"Vendor Serial Number               : CWJH05025502589\n" +
	"\n" +
	"Troubleshooting Info\n" +
	"--------------------\n" +
	"Status Opcode                      : 0\n" +
	"Group Opcode                       : N/A\n" +
	"Recommendation                     : \x1b[31mBad signal integrity\x1b[0m\n"

const mlxlinkSampleHealthyRecommendation = "\n" +
	"Troubleshooting Info\n" +
	"--------------------\n" +
	"Recommendation                     : No issue was observed\n"

// mlxlinkCountersSample mirrors the "Physical Counters and BER Info" section
// emitted by `mlxlink -d <dev> -c` on a healthy 400G link (captured on clnet36
// mlx5_1). mlxlink colorizes the values green on a terminal, so the BER values
// carry ANSI codes here to exercise the stripANSI path.
const mlxlinkCountersSample = "\n" +
	"Physical Counters and BER Info\n" +
	"------------------------------\n" +
	"Time Since Last Clear [Min]        : 348349.5\n" +
	"Effective Physical Errors          : 0\n" +
	"Effective Physical BER             : \x1b[32m15E-255\x1b[0m\n" +
	"Raw Physical Errors Per Lane       : 3946476397,5120336064,7692183445,3026261414\n" +
	"Link Down Counter                  : 0\n" +
	"Link Error Recovery Counter        : 0\n" +
	"Raw Physical BER                   : \x1b[32m2E-9\x1b[0m\n"

// mlxlinkCountersDegradedSample is a synthetic degraded link: FEC no longer
// masks all errors (non-zero Effective errors/BER) and the link has flapped.
const mlxlinkCountersDegradedSample = "\n" +
	"Physical Counters and BER Info\n" +
	"------------------------------\n" +
	"Time Since Last Clear [Min]        : 120.0\n" +
	"Effective Physical Errors          : 42\n" +
	"Effective Physical BER             : 1E-12\n" +
	"Link Down Counter                  : 3\n" +
	"Link Error Recovery Counter        : 7\n" +
	"Raw Physical BER                   : 5E-8\n"

func TestParseMLXLinkCounters(t *testing.T) {
	t.Run("healthy counters with ANSI-colored BER", func(t *testing.T) {
		m := &ModuleInfo{}
		m.parseMLXLinkCounters(mlxlinkCountersSample)
		assert.Equal(t, uint64(0), m.EffectivePhysicalErrors)
		assert.Equal(t, uint64(0), m.LinkDownCounter)
		assert.Equal(t, uint64(0), m.LinkErrorRecoveryCounter)
		assert.InDelta(t, 2e-9, m.RawPhysicalBER, 1e-18)
		assert.InDelta(t, 348349.5, m.TimeSinceLastClearMin, 1e-6)
		// 15E-255 ≈ 0 (FEC nulls it out) but is representable in float64.
		assert.GreaterOrEqual(t, m.EffectivePhysicalBER, 0.0)
		assert.Less(t, m.EffectivePhysicalBER, 1e-100)
	})

	t.Run("degraded counters parse non-zero", func(t *testing.T) {
		m := &ModuleInfo{}
		m.parseMLXLinkCounters(mlxlinkCountersDegradedSample)
		assert.Equal(t, uint64(42), m.EffectivePhysicalErrors)
		assert.Equal(t, uint64(3), m.LinkDownCounter)
		assert.Equal(t, uint64(7), m.LinkErrorRecoveryCounter)
		assert.InDelta(t, 1e-12, m.EffectivePhysicalBER, 1e-20)
		assert.InDelta(t, 5e-8, m.RawPhysicalBER, 1e-16)
		assert.InDelta(t, 120.0, m.TimeSinceLastClearMin, 1e-6)
	})

	t.Run("DOM-only output leaves counters zero", func(t *testing.T) {
		m := &ModuleInfo{}
		m.parseMLXLinkCounters(mlxlinkSampleOutput)
		assert.Equal(t, uint64(0), m.EffectivePhysicalErrors)
		assert.Equal(t, uint64(0), m.LinkDownCounter)
		assert.InDelta(t, 0.0, m.RawPhysicalBER, 1e-18)
	})
}

func TestParseMLXLinkBER(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"2E-9", 2e-9},
		{"15E-255", 15e-255},
		{"1E-12", 1e-12},
		{"0", 0},
		{"N/A", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.InDelta(t, tt.want, parseMLXLinkBER(tt.input), tt.want*1e-6+1e-300)
		})
	}
}

func TestParseMLXLinkUint(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"0", 0},
		{"42", 42},
		{"3946476397", 3946476397},
		{"N/A", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseMLXLinkUint(tt.input))
		})
	}
}

func TestParseMLXLinkRecommendation(t *testing.T) {
	t.Run("bad signal integrity with ANSI color codes", func(t *testing.T) {
		m := &ModuleInfo{}
		m.parseMLXLink(mlxlinkSampleWithRecommendation)
		assert.Equal(t, "Bad signal integrity", m.Recommendation)
	})

	t.Run("healthy recommendation", func(t *testing.T) {
		m := &ModuleInfo{}
		m.parseMLXLink(mlxlinkSampleHealthyRecommendation)
		assert.Equal(t, "No issue was observed", m.Recommendation)
	})

	t.Run("no troubleshooting section leaves recommendation empty", func(t *testing.T) {
		m := &ModuleInfo{}
		m.parseMLXLink(mlxlinkSampleOutput)
		assert.Equal(t, "", m.Recommendation)
	})
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"\x1b[31mBad signal integrity\x1b[0m", "Bad signal integrity"},
		{"No issue was observed", "No issue was observed"},
		{"\x1b[1;33mwarn\x1b[0m", "warn"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, stripANSI(tt.input))
		})
	}
}

func TestParseMLXLinkValueWithRange(t *testing.T) {
	tests := []struct {
		input         string
		wantValue     float64
		wantLow       float64
		wantHigh      float64
	}{
		{"54 [-5..75]", 54, -5, 75},
		{"3261.2 [2970..3630]", 3261.2, 2970, 3630},
		{"1.886,1.989,2.281,1.976 [-10.41..6]", 1.886, -10.41, 6},
		{"1.562,1.427,1.559,1.761 [-8.508..5]", 1.562, -8.508, 5},
		{"8.380,8.380,8.380,8.380 [2..15]", 8.38, 2, 15},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, low, high := parseMLXLinkValueWithRange(tt.input)
			assert.InDelta(t, tt.wantValue, v, 1e-9)
			assert.InDelta(t, tt.wantLow, low, 1e-9)
			assert.InDelta(t, tt.wantHigh, high, 1e-9)
		})
	}
}

func TestParseMLXLinkMultiLane(t *testing.T) {
	tests := []struct {
		input    string
		expected []float64
	}{
		{"8.380,8.380,8.380,8.380 [2..15]", []float64{8.38, 8.38, 8.38, 8.38}},
		{"1.886,1.989,2.281,1.976 [-10.41..6]", []float64{1.886, 1.989, 2.281, 1.976}},
		{"1.562,1.427,1.559,1.761 [-8.508..5]", []float64{1.562, 1.427, 1.559, 1.761}},
		// Single value (no comma)
		{"54 [-5..75]", []float64{54}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseMLXLinkMultiLane(tt.input)
			if assert.Len(t, got, len(tt.expected)) {
				for i, exp := range tt.expected {
					assert.InDelta(t, exp, got[i], 1e-9)
				}
			}
		})
	}
}

func TestParseMLXLink(t *testing.T) {
	m := &ModuleInfo{}
	m.parseMLXLink(mlxlinkSampleOutput)

	assert.Equal(t, "OSFP", m.ModuleType)
	assert.Equal(t, "CLT", m.Vendor)
	assert.Equal(t, "T-F4GS-BV0", m.PartNumber)
	assert.Equal(t, "CWJH05025502589", m.SerialNumber)

	assert.InDelta(t, 54.0, m.Temperature, 1e-9)
	assert.InDelta(t, -5.0, m.TempLowAlarm, 1e-9)
	assert.InDelta(t, 75.0, m.TempHighAlarm, 1e-9)

	// Voltage: 3261.2 mV → 3.2612 V
	assert.InDelta(t, 3.2612, m.Voltage, 1e-9)

	if assert.Len(t, m.BiasCurrent, 4) {
		for _, bc := range m.BiasCurrent {
			assert.InDelta(t, 8.38, bc, 1e-9)
		}
	}

	if assert.Len(t, m.RxPower, 4) {
		assert.InDelta(t, 1.886, m.RxPower[0], 1e-9)
		assert.InDelta(t, 1.989, m.RxPower[1], 1e-9)
		assert.InDelta(t, 2.281, m.RxPower[2], 1e-9)
		assert.InDelta(t, 1.976, m.RxPower[3], 1e-9)
	}
	assert.InDelta(t, -10.41, m.RxPowerLowAlarm, 1e-9)
	assert.InDelta(t, 6.0, m.RxPowerHighAlarm, 1e-9)

	if assert.Len(t, m.TxPower, 4) {
		assert.InDelta(t, 1.562, m.TxPower[0], 1e-9)
		assert.InDelta(t, 1.427, m.TxPower[1], 1e-9)
		assert.InDelta(t, 1.559, m.TxPower[2], 1e-9)
		assert.InDelta(t, 1.761, m.TxPower[3], 1e-9)
	}
	assert.InDelta(t, -8.508, m.TxPowerLowAlarm, 1e-9)
	assert.InDelta(t, 5.0, m.TxPowerHighAlarm, 1e-9)
}
