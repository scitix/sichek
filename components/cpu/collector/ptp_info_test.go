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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPTPInfoGet(t *testing.T) {
	p := &PTPInfo{}
	// Get should not panic even when services are not installed
	p.Get()
	// SyncAvailable may be false if neither PTP nor NTP is running
	// Just verify the struct is populated without error
	assert.IsType(t, false, p.PTPServiceActive)
	assert.IsType(t, float64(0), p.OffsetNs)
}

func TestParsePTP4LOffset(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{
			name:  "valid positive offset",
			input: "ptp4l[1234.567]: master offset        123 s2 freq   +1234 path delay       456",
			want:  123,
		},
		{
			name:  "valid negative offset",
			input: "ptp4l[1234.567]: master offset        -42 s2 freq   +1234 path delay       456",
			want:  -42,
		},
		{
			name: "multiple lines uses last",
			input: `ptp4l[1.0]: master offset        100 s2 freq   +0 path delay       0
ptp4l[2.0]: master offset        200 s2 freq   +0 path delay       0`,
			want: 200,
		},
		{
			name:    "no master offset",
			input:   "some unrelated log line",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePTP4LOffset(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseChronycOffset(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{
			name: "valid positive offset",
			input: `Reference ID    : A1B2C3D4 (ntp.example.com)
Stratum         : 3
Last offset     : +0.000012345 seconds
RMS offset      : 0.000023456 seconds`,
			want: 12345.0,
		},
		{
			name: "valid negative offset",
			input: `Reference ID    : A1B2C3D4 (ntp.example.com)
Last offset     : -0.000001000 seconds
RMS offset      : 0.000002000 seconds`,
			want: -1000.0,
		},
		{
			name:    "missing Last offset",
			input:   "Reference ID    : A1B2C3D4\nStratum         : 3\n",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChronycOffset(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestPTPInfoOffsetMs(t *testing.T) {
	p := &PTPInfo{
		PTPServiceActive: true,
		OffsetNs:         -1500000,
	}
	assert.InDelta(t, 1.5, p.OffsetMs(), 0.001)

	p2 := &PTPInfo{
		PTPServiceActive: false,
		NTPOffset:        2000000,
	}
	assert.InDelta(t, 2.0, p2.OffsetMs(), 0.001)

	// Zero offset
	p3 := &PTPInfo{}
	assert.Equal(t, 0.0, math.Abs(p3.OffsetMs()))
}
