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

func TestMCEInfoGetFromSysfs(t *testing.T) {
	dir := t.TempDir()

	// Create two machinecheck directories with count files
	for _, name := range []string{"machinecheck0", "machinecheck1"} {
		mcDir := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(mcDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(mcDir, "corrected_count"), []byte("3\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(mcDir, "uncorrected_count"), []byte("1\n"), 0644))
	}

	m := &MCEInfo{}
	m.getFromDir(dir)

	assert.True(t, m.Available)
	assert.Equal(t, int64(6), m.CorrectedCount)   // 3 + 3
	assert.Equal(t, int64(2), m.UncorrectedCount) // 1 + 1
}

func TestMCEInfoGetFromSysfs_NotAvailable(t *testing.T) {
	dir := t.TempDir()
	// Empty directory - no machinecheck dirs

	m := &MCEInfo{}
	m.getFromDir(dir)

	assert.False(t, m.Available)
	assert.Equal(t, int64(0), m.CorrectedCount)
	assert.Equal(t, int64(0), m.UncorrectedCount)
}

func TestMCEInfoGetFromSysfs_MissingFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a machinecheck directory without count files
	mcDir := filepath.Join(dir, "machinecheck0")
	require.NoError(t, os.MkdirAll(mcDir, 0755))

	m := &MCEInfo{}
	m.getFromDir(dir)

	assert.True(t, m.Available)
	assert.Equal(t, int64(0), m.CorrectedCount)
	assert.Equal(t, int64(0), m.UncorrectedCount)
}

func TestMCEInfoGetFromSysfs_NonexistentDir(t *testing.T) {
	m := &MCEInfo{}
	m.getFromDir("/nonexistent/path/machinecheck")

	assert.False(t, m.Available)
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
