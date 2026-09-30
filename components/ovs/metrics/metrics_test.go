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
package metrics

import (
	"testing"

	"github.com/scitix/sichek/components/ovs/collector"
)

func TestExportMetrics_DoesNotPanic(t *testing.T) {
	m := NewOVSMetrics()
	m.ExportMetrics(&collector.OVSInfo{Available: false})
	m.ExportMetrics(&collector.OVSInfo{
		Available: true,
		Services:  map[string]string{"ovs-vswitchd": "active"},
		Bridges: []collector.BridgeInfo{
			{
				Name:     "br-rail0",
				Ports:    5,
				Flows:    18,
				GroupIDs: []int{10},
				PortDetails: []collector.PortInfo{
					{
						Name:      "p0",
						OFPort:    1,
						RxBytes:   1024,
						TxBytes:   2048,
						RxErrPkts: 3,
					},
				},
			},
		},
		Datapath: collector.DatapathInfo{Name: "doca@ovs-doca", LookupsHit: 5},
		Coverage: map[string]uint64{"flow_offload_200ms_latency": 1},
	})
}
