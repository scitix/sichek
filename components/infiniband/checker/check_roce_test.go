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
package checker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoCEChecker_ConnectivitySnapshot exercises the gateway-classification and
// snapshot-embedding logic without any network I/O: both Ethernet devices have
// a gateway that needs no probe (the "IPV6" sentinel and an empty L2-only
// gateway), so the result is deterministic on any host.
func TestRoCEChecker_ConnectivitySnapshot(t *testing.T) {
	info := &collector.InfinibandInfo{
		IBNicRole: "macvlanNode",
		IBHardWareInfo: map[string]collector.IBHardWareInfo{
			"mlx5_0": {IBDev: "mlx5_0", NetDev: "eth0", LinkLayer: "Ethernet", PFGW: "IPV6"},
			"mlx5_1": {IBDev: "mlx5_1", NetDev: "eth1", LinkLayer: "Ethernet", PFGW: ""},
			"mezz_0": {IBDev: "mezz_0", NetDev: "ib0", LinkLayer: "InfiniBand", PFGW: ""},
		},
	}

	chk, err := NewRoCEChecker(&config.InfinibandSpec{})
	require.NoError(t, err)

	res, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, res.Status)
	// Ethernet devices present but none had an IPv4 gateway to probe.
	assert.Equal(t, "N/A", res.Curr)

	// Connectivity is written back into the collector info, which is exactly what
	// the snapshot persists (daemon stores LastInfo()).
	require.NotNil(t, info.RoCEConnectivity)
	assert.Len(t, info.RoCEConnectivity, 2, "only Ethernet devices, InfiniBand excluded")
	for _, dev := range []string{"mlx5_0", "mlx5_1"} {
		st := info.RoCEConnectivity[dev]
		require.NotNilf(t, st, "missing connectivity for %s", dev)
		assert.Equal(t, "skipped", st.State)
		assert.Equal(t, int64(0), st.LatencyUs)
	}
	_, hasIB := info.RoCEConnectivity["mezz_0"]
	assert.False(t, hasIB, "InfiniBand device must not appear in RoCE connectivity")

	// The data must survive JSON serialization — that is the snapshot format.
	blob, err := json.Marshal(info)
	require.NoError(t, err)
	assert.Contains(t, string(blob), "roce_connectivity")
	assert.Contains(t, string(blob), "latency_us")
}
