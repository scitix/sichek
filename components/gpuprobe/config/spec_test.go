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
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSpec_EmptyReturnsDefault(t *testing.T) {
	s, err := LoadSpec("")
	require.NoError(t, err)
	assert.Equal(t, "/var/sichek/bin/gpu_probe", s.ProbeBinaryPath)
	assert.Equal(t, 30, s.ProbeTimeoutSec)
	assert.Equal(t, 1, s.FailConsecutiveThreshold)
	assert.Equal(t, consts.LevelCritical, s.FailLevel)
}

func TestLoadSpec_InvalidFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(p, []byte("gpuprobe:\n  probe_timeout_sec: 0\n"), 0644))
	s, err := LoadSpec(p)
	require.NoError(t, err)
	assert.Equal(t, 30, s.ProbeTimeoutSec) // 回落默认
}

func TestLoadSpec_ValidOverrides(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "ok.yaml")
	yaml := "gpuprobe:\n  probe_binary_path: /opt/gp\n  probe_timeout_sec: 15\n  min_free_mem_pct: 70\n"
	require.NoError(t, os.WriteFile(p, []byte(yaml), 0644))
	s, err := LoadSpec(p)
	require.NoError(t, err)
	assert.Equal(t, "/opt/gp", s.ProbeBinaryPath)
	assert.Equal(t, 15, s.ProbeTimeoutSec)
	assert.Equal(t, 70, s.MinFreeMemPct)
}
