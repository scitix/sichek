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
