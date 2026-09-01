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

func TestParseMetricsSample(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "metrics_sample.txt"))
	require.NoError(t, err)

	byName := ParseMetrics(string(body), "rdma_env_pre_")

	// non-prefixed series filtered out
	assert.NotContains(t, byName, "other_exporter_metric")
	// malformed line skipped
	assert.NotContains(t, byName, "rdma_env_pre_broken_line")

	// knob_verdict: 4 series, labels parsed
	verdicts := byName["rdma_env_pre_knob_verdict"]
	require.Len(t, verdicts, 4)
	assert.Equal(t, "0000:19:00.0", verdicts[0].Labels["device"])
	assert.Equal(t, "converged", verdicts[0].Labels["verdict"])
	assert.Equal(t, "true", verdicts[3].Labels["reboot_required"])
	assert.Equal(t, float64(1), verdicts[0].Value)

	// numeric value families
	obs := byName["rdma_env_pre_knob_observed_value"]
	require.Len(t, obs, 3)
	assert.Equal(t, float64(3), findByDevice(obs, "0000:ef:00.3").Value)

	// info family (non-numeric values carried in labels)
	info := byName["rdma_env_pre_knob_info"]
	require.Len(t, info, 1)
	assert.Equal(t, "dscp", info[0].Labels["desired"])
	assert.Equal(t, "dscp", info[0].Labels["observed"])

	// no-label series
	up := byName["rdma_env_pre_agent_up"]
	require.Len(t, up, 1)
	assert.Empty(t, up[0].Labels)
	assert.Equal(t, float64(1), up[0].Value)

	// label with a value on a no-fabric family
	build := byName["rdma_env_pre_build_info"]
	require.Len(t, build, 1)
	assert.Equal(t, "1.9.1-knobtest", build[0].Labels["version"])

	// host_compliance state
	hc := byName["rdma_env_pre_host_compliance"]
	require.Len(t, hc, 1)
	assert.Equal(t, "drift", hc[0].Labels["state"])
}

func TestParseMetricsEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantName   string
		wantValue  float64
		wantLabels map[string]string
	}{
		{
			name:       "no labels",
			line:       "rdma_env_pre_agent_up 1",
			wantOK:     true,
			wantName:   "rdma_env_pre_agent_up",
			wantValue:  1,
			wantLabels: map[string]string{},
		},
		{
			name:       "trailing timestamp ignored",
			line:       `rdma_env_pre_x{a="b"} 5 1712345678`,
			wantOK:     true,
			wantName:   "rdma_env_pre_x",
			wantValue:  5,
			wantLabels: map[string]string{"a": "b"},
		},
		{
			name:       "escaped quote and comma inside value",
			line:       `rdma_env_pre_x{desc="a\"b,c",k="v"} 1`,
			wantOK:     true,
			wantName:   "rdma_env_pre_x",
			wantValue:  1,
			wantLabels: map[string]string{"desc": `a"b,c`, "k": "v"},
		},
		{
			name:   "no value",
			line:   `rdma_env_pre_x{a="b"}`,
			wantOK: false,
		},
		{
			name:   "bad value",
			line:   `rdma_env_pre_x notanumber`,
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, labels, value, ok := parseLine(tt.line)
			assert.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantValue, value)
			assert.Equal(t, tt.wantLabels, labels)
		})
	}
}

func findByDevice(series []Series, device string) Series {
	for _, s := range series {
		if s.Labels["device"] == device {
			return s
		}
	}
	return Series{}
}
