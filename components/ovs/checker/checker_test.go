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
	"testing"

	"github.com/scitix/sichek/components/ovs/collector"
	"github.com/scitix/sichek/components/ovs/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
)

func healthyInfo() *collector.OVSInfo {
	return &collector.OVSInfo{
		Available: true,
		Services: map[string]string{
			"openvswitch-switch": "active", "ovs-vswitchd": "active", "ovsdb-server": "active",
		},
		Packages: map[string]string{
			"doca-openvswitch-switch": "3.3.0040-1", "doca-openvswitch-common": "3.3.0040-1",
			"collectx-clxapi": "1.24.3", "libnvhws1": "26.01.9-1",
		},
		OVSVersion: "3.3.0040", DPDKVersion: "25.11.0", DPDKInitialized: true,
		OtherConfig: map[string]string{
			"doca-init": "true", "hw-offload": "true", "hw-offload-ct-size": "0",
			"max-idle": "300000", "doca-eswitch-max": "4",
		},
		Bridges: func() []collector.BridgeInfo {
			var bs []collector.BridgeInfo
			for r := 0; r < 8; r++ {
				bs = append(bs, collector.BridgeInfo{
					Name: "br-rail" + string(rune('0'+r)), Exists: true, DatapathType: "netdev",
					Ports: 5, Flows: 18, GroupIDs: []int{10, 20, 21, 22, 23, 30, 31, 32, 33},
				})
			}
			return bs
		}(),
	}
}

func TestServiceChecker_Healthy(t *testing.T) {
	c := &ServiceChecker{}
	r, err := c.Check(context.Background(), healthyInfo())
	assert.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, r.Status)
}

func TestServiceChecker_VswitchdDown(t *testing.T) {
	info := healthyInfo()
	info.Services["ovs-vswitchd"] = "inactive"
	r, _ := (&ServiceChecker{}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestOtherConfigChecker_Mismatch(t *testing.T) {
	info := healthyInfo()
	info.OtherConfig["hw-offload"] = "false"
	r, _ := (&OtherConfigChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestVersionChecker_PackageMissing(t *testing.T) {
	info := healthyInfo()
	info.Packages["libnvhws1"] = ""
	r, _ := (&VersionChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestVersionChecker_RuntimeEmptyIsWarning(t *testing.T) {
	info := healthyInfo()
	info.OVSVersion = ""
	r, _ := (&VersionChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelWarning, r.Level)
}

func TestBridgeChecker_MissingGroupID(t *testing.T) {
	info := healthyInfo()
	info.Bridges[0].GroupIDs = []int{10, 20, 21} // missing 22,23,30-33
	r, _ := (&BridgeChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestBridgeChecker_OrphanPortsIsWarning(t *testing.T) {
	info := healthyInfo()
	info.Bridges[0].OrphanPorts = []int{6}
	r, _ := (&BridgeChecker{spec: config.DefaultSpec()}).Check(context.Background(), info)
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelWarning, r.Level)
}

func TestBridgeChecker_Healthy(t *testing.T) {
	r, _ := (&BridgeChecker{spec: config.DefaultSpec()}).Check(context.Background(), healthyInfo())
	assert.Equal(t, consts.StatusNormal, r.Status)
}
