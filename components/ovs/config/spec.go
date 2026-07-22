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
	"github.com/scitix/sichek/components/common"
	"github.com/sirupsen/logrus"
)

type OVSSpecConfig struct {
	OVS *OVSSpec `json:"ovs" yaml:"ovs"`
}

type OVSSpec struct {
	BridgePrefix     string            `json:"bridge_prefix" yaml:"bridge_prefix"`
	NumRails         int               `json:"num_rails" yaml:"num_rails"`
	PortsPerBridge   int               `json:"ports_per_bridge" yaml:"ports_per_bridge"`
	MinFlows         int               `json:"min_flows" yaml:"min_flows"`
	ExpectedGroupIDs []int             `json:"expected_group_ids" yaml:"expected_group_ids"`
	DatapathType     string            `json:"datapath_type" yaml:"datapath_type"`
	OtherConfig      map[string]string `json:"other_config" yaml:"other_config"`
	RequiredPackages []string          `json:"required_packages" yaml:"required_packages"`
	CoverageEvents   []string          `json:"coverage_events" yaml:"coverage_events"`
}

// valid reports whether a loaded spec has our required flat-schema fields
// populated. A legacy/stale `ovs:` section with a different nested schema
// unmarshals into a zero-valued OVSSpec (NumRails==0, etc.) and must be
// rejected so we fall back to the built-in default rather than checking
// nothing. The nil receiver check makes valid() safe to call on a nil *OVSSpec.
func (s *OVSSpec) valid() bool {
	return s != nil && s.NumRails > 0 && s.BridgePrefix != "" && len(s.ExpectedGroupIDs) > 0
}

// LoadSpec loads the OVS spec from file; on any failure, or when the loaded
// spec is empty/legacy-shaped, it returns the built-in default.
func LoadSpec(file string) (*OVSSpec, error) {
	if file == "" {
		return DefaultSpec(), nil
	}
	var s OVSSpecConfig
	if err := common.LoadSpec(file, &s); err != nil || !s.OVS.valid() {
		logrus.WithField("component", "ovs/spec").Warnf("ovs spec in %s missing/invalid (err=%v), using built-in default", file, err)
		return DefaultSpec(), nil
	}
	return s.OVS, nil
}

// DefaultSpec mirrors the rdma_env_vv ovs hardcoded baselines (Step 5/8/10).
func DefaultSpec() *OVSSpec {
	return &OVSSpec{
		BridgePrefix:     "br-rail",
		NumRails:         8,
		PortsPerBridge:   5,
		MinFlows:         18,
		ExpectedGroupIDs: []int{10, 20, 21, 22, 23, 30, 31, 32, 33},
		DatapathType:     "netdev",
		OtherConfig: map[string]string{
			"doca-init":          "true",
			"hw-offload":         "true",
			"hw-offload-ct-size": "0",
			"max-idle":           "300000",
			"doca-eswitch-max":   "4",
		},
		RequiredPackages: []string{
			"doca-openvswitch-switch",
			"doca-openvswitch-common",
			"collectx-clxapi",
			"libnvhws1",
		},
		CoverageEvents: []string{
			"flow_offload_200ms_latency",
			"doca_datapath_drop_upcall_error",
		},
	}
}
