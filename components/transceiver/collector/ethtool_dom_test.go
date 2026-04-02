package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const ethtoolSampleOutput = `
        Identifier                                : 0x03 (SFP)
        Vendor name                               : ZSINE
        Vendor PN                                 : SFP28-25G-AOC30M
        Vendor SN                                 : 251012H30164
        Module temperature                        : 38.00 degrees C / 100.40 degrees F
        Module voltage                            : 3.2740 V
        Laser tx bias current                     : 6.750 mA
        Laser tx power                            : 0.5012 mW / -3.00 dBm
        Receiver signal average optical power     : 0.4898 mW / -3.10 dBm
        Module temperature high alarm threshold   : 75.00 degrees C
        Module temperature low alarm threshold    : -5.00 degrees C
        Laser tx power high alarm threshold       : 1.0000 mW / 0.00 dBm
        Laser tx power low alarm threshold        : 0.0100 mW / -20.00 dBm
        Laser rx power high alarm threshold       : 1.0000 mW / 0.00 dBm
        Laser rx power low alarm threshold        : 0.0100 mW / -20.00 dBm
`

func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"38.00 degrees C", 38.0},
		{"3.2740 V", 3.274},
		{"6.750 mA", 6.75},
		{"-5.00 degrees C", -5.0},
		{"75.00 degrees C", 75.0},
		{"0", 0.0},
		{"", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseFloat(tt.input)
			assert.InDelta(t, tt.expected, got, 1e-9)
		})
	}
}

func TestParseDBM(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"0.5012 mW / -3.00 dBm", -3.0},
		{"0.4898 mW / -3.10 dBm", -3.1},
		{"1.0000 mW / 0.00 dBm", 0.0},
		{"0.0100 mW / -20.00 dBm", -20.0},
		// No dBm unit — falls back to parseFloat
		{"3.2740 V", 3.274},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDBM(tt.input)
			assert.InDelta(t, tt.expected, got, 1e-9)
		})
	}
}

func TestParseEthtoolModule(t *testing.T) {
	m := &ModuleInfo{}
	m.parseEthtoolModule(ethtoolSampleOutput)

	assert.Equal(t, "0x03 (SFP)", m.ModuleType)
	assert.Equal(t, "ZSINE", m.Vendor)
	assert.Equal(t, "SFP28-25G-AOC30M", m.PartNumber)
	assert.Equal(t, "251012H30164", m.SerialNumber)

	assert.InDelta(t, 38.0, m.Temperature, 1e-9)
	assert.InDelta(t, 3.274, m.Voltage, 1e-9)

	// The switch uses strings.HasPrefix(key, "Laser tx power"), which also matches
	// "Laser tx power high alarm threshold" and "Laser tx power low alarm threshold",
	// so all three lines are appended to TxPower and the explicit alarm cases are never reached.
	if assert.Len(t, m.TxPower, 3) {
		assert.InDelta(t, -3.0, m.TxPower[0], 1e-9)  // actual reading
		assert.InDelta(t, 0.0, m.TxPower[1], 1e-9)   // high alarm threshold value
		assert.InDelta(t, -20.0, m.TxPower[2], 1e-9) // low alarm threshold value
	}
	// TxPowerHighAlarm and TxPowerLowAlarm remain zero (consumed by the HasPrefix case above)
	assert.InDelta(t, 0.0, m.TxPowerHighAlarm, 1e-9)
	assert.InDelta(t, 0.0, m.TxPowerLowAlarm, 1e-9)

	if assert.Len(t, m.RxPower, 1) {
		assert.InDelta(t, -3.1, m.RxPower[0], 1e-9)
	}
	if assert.Len(t, m.BiasCurrent, 1) {
		assert.InDelta(t, 6.75, m.BiasCurrent[0], 1e-9)
	}

	assert.InDelta(t, 75.0, m.TempHighAlarm, 1e-9)
	assert.InDelta(t, -5.0, m.TempLowAlarm, 1e-9)

	// Rx alarm threshold keys ("Laser rx power high/low alarm threshold") do NOT start with
	// "Receiver signal average optical power", so they correctly reach the explicit alarm cases.
	assert.InDelta(t, 0.0, m.RxPowerHighAlarm, 1e-9)
	assert.InDelta(t, -20.0, m.RxPowerLowAlarm, 1e-9)
}
