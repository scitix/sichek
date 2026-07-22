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
	"encoding/json"
	"time"
)

type OVSInfo struct {
	Time            time.Time         `json:"time"`
	Available       bool              `json:"available"`
	SkipReason      string            `json:"skip_reason,omitempty"`
	Services        map[string]string `json:"services"` // svc -> systemctl state
	Packages        map[string]string `json:"packages"` // pkg -> version ("" = not installed)
	OVSVersion      string            `json:"ovs_version"`
	DPDKVersion     string            `json:"dpdk_version"`
	DPDKInitialized bool              `json:"dpdk_initialized"`
	OtherConfig     map[string]string `json:"other_config"`
	Bridges         []BridgeInfo      `json:"bridges"`
	Datapath        DatapathInfo      `json:"datapath"`
	Coverage        map[string]uint64 `json:"coverage"`
}

type BridgeInfo struct {
	Name           string     `json:"name"`
	Exists         bool       `json:"exists"`
	DatapathType   string     `json:"datapath_type"`
	FailMode       string     `json:"fail_mode"`
	Ports          int        `json:"ports"`
	Flows          int        `json:"flows"`
	GroupIDs       []int      `json:"group_ids"`
	OrphanFlowRefs []int      `json:"orphan_flow_refs"`
	OrphanPorts    []int      `json:"orphan_ports"`
	PortDetails    []PortInfo `json:"port_details"`
}

type PortInfo struct {
	Name       string `json:"name"`
	OFPort     int    `json:"ofport"`
	AdminState string `json:"admin_state"`
	LinkState  string `json:"link_state"`
	Error      string `json:"error,omitempty"`
	RxBytes    uint64 `json:"rx_bytes"`
	TxBytes    uint64 `json:"tx_bytes"`
	RxErrPkts  uint64 `json:"rx_err_pkts"`
}

type DatapathInfo struct {
	Name          string    `json:"name"`
	DPFlows       int       `json:"dp_flows"`
	LookupsHit    uint64    `json:"lookups_hit"`
	LookupsMissed uint64    `json:"lookups_missed"`
	LookupsLost   uint64    `json:"lookups_lost"`
	PMDs          []PMDInfo `json:"pmds"`
}

type PMDInfo struct {
	Core             string  `json:"core"`
	NUMA             string  `json:"numa"`
	BusyRatio        float64 `json:"busy_ratio"`
	IdleCycles       uint64  `json:"idle_cycles"`
	ProcessingCycles uint64  `json:"processing_cycles"`
	RxPackets        uint64  `json:"rx_packets"`
}

// JSON satisfies common.Info.
func (o *OVSInfo) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}
