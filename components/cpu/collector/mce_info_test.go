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

// writeEDACMC creates an EDAC memory-controller directory (mcN) holding the
// given count files; a nil value leaves that file out.
func writeEDACMC(t *testing.T, dir, name string, ce, ue *string) {
	t.Helper()
	mcDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(mcDir, 0755))
	if ce != nil {
		require.NoError(t, os.WriteFile(filepath.Join(mcDir, "ce_count"), []byte(*ce), 0644))
	}
	if ue != nil {
		require.NoError(t, os.WriteFile(filepath.Join(mcDir, "ue_count"), []byte(*ue), 0644))
	}
}

func strPtr(s string) *string { return &s }

func TestMCEInfoDefaultPathIsEDAC(t *testing.T) {
	assert.Equal(t, "/sys/devices/system/edac/mc", edacMCPath)
}

func TestMCEInfoGetFromDir(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, dir string)
		wantAvailable bool
		wantCorrected int64
		wantUncorr    int64
	}{
		{
			name: "sums all memory controllers",
			setup: func(t *testing.T, dir string) {
				writeEDACMC(t, dir, "mc0", strPtr("3\n"), strPtr("1\n"))
				writeEDACMC(t, dir, "mc1", strPtr("4\n"), strPtr("0\n"))
			},
			wantAvailable: true,
			wantCorrected: 7,
			wantUncorr:    1,
		},
		{
			name: "healthy node reports available with zero counts",
			setup: func(t *testing.T, dir string) {
				writeEDACMC(t, dir, "mc0", strPtr("0\n"), strPtr("0\n"))
			},
			wantAvailable: true,
		},
		{
			name: "ignores non-mc entries such as power and uevent",
			setup: func(t *testing.T, dir string) {
				writeEDACMC(t, dir, "mc0", strPtr("2\n"), strPtr("0\n"))
				writeEDACMC(t, dir, "power", strPtr("99\n"), strPtr("99\n"))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "uevent"), nil, 0644))
			},
			wantAvailable: true,
			wantCorrected: 2,
		},
		{
			name:          "no memory controllers is not available",
			setup:         func(t *testing.T, dir string) {},
			wantAvailable: false,
		},
		{
			name: "mc dir without count files is not available",
			setup: func(t *testing.T, dir string) {
				writeEDACMC(t, dir, "mc0", nil, nil)
			},
			wantAvailable: false,
		},
		{
			name: "unparsable count is not available",
			setup: func(t *testing.T, dir string) {
				writeEDACMC(t, dir, "mc0", strPtr("garbage\n"), strPtr("0\n"))
			},
			wantAvailable: false,
		},
		{
			name: "one readable controller keeps the node available",
			setup: func(t *testing.T, dir string) {
				writeEDACMC(t, dir, "mc0", nil, nil)
				writeEDACMC(t, dir, "mc1", strPtr("5\n"), strPtr("2\n"))
			},
			wantAvailable: true,
			wantCorrected: 5,
			wantUncorr:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			m := &MCEInfo{}
			m.getFromDir(dir)

			assert.Equal(t, tt.wantAvailable, m.Available)
			assert.Equal(t, tt.wantCorrected, m.CorrectedCount)
			assert.Equal(t, tt.wantUncorr, m.UncorrectedCount)
		})
	}
}

func TestMCEInfoGetFromDir_NonexistentDir(t *testing.T) {
	m := &MCEInfo{}
	m.getFromDir("/nonexistent/path/edac/mc")

	assert.False(t, m.Available)
}

func TestMCEInfoGetResetsPreviousValues(t *testing.T) {
	dir := t.TempDir()
	writeEDACMC(t, dir, "mc0", strPtr("3\n"), strPtr("1\n"))

	m := &MCEInfo{}
	m.getFromDir(dir)
	m.getFromDir(dir)

	assert.Equal(t, int64(3), m.CorrectedCount)
	assert.Equal(t, int64(1), m.UncorrectedCount)
}

func TestReadCountFile(t *testing.T) {
	dir := t.TempDir()

	validPath := filepath.Join(dir, "valid")
	require.NoError(t, os.WriteFile(validPath, []byte("42\n"), 0644))
	v, err := readCountFile(validPath)
	require.NoError(t, err)
	assert.Equal(t, int64(42), v)

	invalidPath := filepath.Join(dir, "invalid")
	require.NoError(t, os.WriteFile(invalidPath, []byte("not_a_number\n"), 0644))
	_, err = readCountFile(invalidPath)
	assert.Error(t, err)

	_, err = readCountFile(filepath.Join(dir, "missing"))
	assert.Error(t, err)
}

func TestReadIntFile(t *testing.T) {
	dir := t.TempDir()

	// Valid file
	validPath := filepath.Join(dir, "valid")
	require.NoError(t, os.WriteFile(validPath, []byte("42\n"), 0644))
	assert.Equal(t, int64(42), readIntFile(validPath))

	// Invalid content
	invalidPath := filepath.Join(dir, "invalid")
	require.NoError(t, os.WriteFile(invalidPath, []byte("not_a_number\n"), 0644))
	assert.Equal(t, int64(0), readIntFile(invalidPath))

	// Missing file
	assert.Equal(t, int64(0), readIntFile(filepath.Join(dir, "missing")))
}
