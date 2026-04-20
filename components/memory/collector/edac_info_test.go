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

func TestEDACInfoGetFromDir(t *testing.T) {
	// Create a fake EDAC sysfs tree:
	//   <tmpdir>/mc/mc0/ce_count, ue_count, csrow0/{ce_count,ue_count}
	//   <tmpdir>/mc/mc1/ce_count, ue_count, csrow0/{ce_count,ue_count}
	tmpDir := t.TempDir()
	mcBase := filepath.Join(tmpDir, "mc")

	// mc0: ce=5, ue=1, csrow0: ce=3, ue=1
	mc0 := filepath.Join(mcBase, "mc0")
	csrow0 := filepath.Join(mc0, "csrow0")
	require.NoError(t, os.MkdirAll(csrow0, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mc0, "ce_count"), []byte("5\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(mc0, "ue_count"), []byte("1\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow0, "ce_count"), []byte("3\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow0, "ue_count"), []byte("1\n"), 0644))

	// mc1: ce=2, ue=0, csrow0: ce=2, ue=0
	mc1 := filepath.Join(mcBase, "mc1")
	csrow1 := filepath.Join(mc1, "csrow0")
	require.NoError(t, os.MkdirAll(csrow1, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(mc1, "ce_count"), []byte("2\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(mc1, "ue_count"), []byte("0\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow1, "ce_count"), []byte("2\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(csrow1, "ue_count"), []byte("0\n"), 0644))

	edac := &EDACInfo{}
	edac.getFromDir(tmpDir)

	assert.True(t, edac.Available)
	assert.Equal(t, int64(7), edac.TotalCE)
	assert.Equal(t, int64(1), edac.TotalUCE)
	assert.Len(t, edac.Controllers, 2)

	// Verify mc0
	assert.Equal(t, "mc0", edac.Controllers[0].ID)
	assert.Equal(t, int64(5), edac.Controllers[0].CECount)
	assert.Equal(t, int64(1), edac.Controllers[0].UCECount)
	assert.Len(t, edac.Controllers[0].CSRows, 1)
	assert.Equal(t, int64(3), edac.Controllers[0].CSRows[0].CECount)

	// Verify mc1
	assert.Equal(t, "mc1", edac.Controllers[1].ID)
	assert.Equal(t, int64(2), edac.Controllers[1].CECount)
	assert.Equal(t, int64(0), edac.Controllers[1].UCECount)
}

func TestEDACInfoGetFromDir_NotAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	// No mc/ directory exists under tmpDir

	edac := &EDACInfo{}
	edac.getFromDir(tmpDir)

	assert.False(t, edac.Available)
	assert.Nil(t, edac.Controllers)
	assert.Equal(t, int64(0), edac.TotalCE)
	assert.Equal(t, int64(0), edac.TotalUCE)
}
