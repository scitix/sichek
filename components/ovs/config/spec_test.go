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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempSpec(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ovs_spec.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadSpec_EmptyFileReturnsDefault(t *testing.T) {
	spec, err := LoadSpec("")
	assert.NoError(t, err)
	assert.Equal(t, 8, spec.NumRails)
	assert.Equal(t, 5, spec.PortsPerBridge)
	assert.Equal(t, 18, spec.MinFlows)
	assert.Equal(t, []int{10, 20, 21, 22, 23, 30, 31, 32, 33}, spec.ExpectedGroupIDs)
	assert.Equal(t, "true", spec.OtherConfig["hw-offload"])
	assert.Contains(t, spec.RequiredPackages, "libnvhws1")
}

func TestLoadSpec_LegacyNestedShapeFallsBackToDefault(t *testing.T) {
	legacy := `ovs:
  default:
    expected_bridges: [br-rail0, br-rail1, br-rail2, br-rail3, br-rail4, br-rail5, br-rail6, br-rail7]
    per_bridge:
      datapath_type: "netdev"
      fail_mode: "secure"
      min_dpdk_ports: 4
      require_vf_representor: true
    hw_offload: true
    doca_version_min: "3.3.0040"
`
	path := writeTempSpec(t, legacy)
	spec, err := LoadSpec(path)
	assert.NoError(t, err)
	assert.Equal(t, 8, spec.NumRails)
	assert.Len(t, spec.ExpectedGroupIDs, 9)
	assert.Equal(t, "true", spec.OtherConfig["hw-offload"])
}

func TestLoadSpec_ValidFlatSpec(t *testing.T) {
	flat := `ovs:
  bridge_prefix: "br-rail"
  num_rails: 4
  ports_per_bridge: 5
  min_flows: 18
  expected_group_ids: [10, 20]
  datapath_type: "netdev"
`
	path := writeTempSpec(t, flat)
	spec, err := LoadSpec(path)
	assert.NoError(t, err)
	assert.Equal(t, 4, spec.NumRails)
	assert.Equal(t, "br-rail", spec.BridgePrefix)
	assert.Equal(t, 5, spec.PortsPerBridge)
	assert.Equal(t, 18, spec.MinFlows)
	assert.Equal(t, []int{10, 20}, spec.ExpectedGroupIDs)
	assert.Equal(t, "netdev", spec.DatapathType)
}
