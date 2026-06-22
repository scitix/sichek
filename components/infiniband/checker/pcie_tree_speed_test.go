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
	"strings"
	"testing"

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildHW(dev, bdf string, links []collector.PCIETreeLink) collector.IBHardWareInfo {
	return collector.IBHardWareInfo{
		IBDev:         dev,
		PCIEBDF:       bdf,
		PCIETreeLinks: links,
	}
}

// runSpeedChecker constructs an InfinibandInfo from the given map of IBDev →
// IBHardWareInfo and runs the checker. The unexported sync.RWMutex inside
// InfinibandInfo is zero-value usable, so we don't touch it.
func runSpeedChecker(t *testing.T, hws map[string]collector.IBHardWareInfo) (string, string, string, string) {
	t.Helper()
	ck, err := NewIBPCIETreeSpeedChecker(&config.InfinibandSpec{})
	require.NoError(t, err)
	info := &collector.InfinibandInfo{IBHardWareInfo: hws}
	res, err := ck.Check(context.Background(), info)
	require.NoError(t, err)
	return res.Status, res.Device, res.Curr, res.Spec
}

func TestIBPCIETreeSpeedChecker_NoLinksReportsNormal(t *testing.T) {
	status, dev, curr, spec := runSpeedChecker(t, map[string]collector.IBHardWareInfo{
		"mlx5_0": buildHW("mlx5_0", "0000:82:00.0", nil),
	})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
	assert.Empty(t, curr)
	assert.Empty(t, spec)
}

func TestIBPCIETreeSpeedChecker_FullSpeedNormal(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
}

func TestIBPCIETreeSpeedChecker_RootGen4_NoAlarm(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "16.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, _, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
}

func TestIBPCIETreeSpeedChecker_OneLinkDegraded(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:81:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:81:00.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, curr, spec := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Contains(t, dev, "mlx5_0(0000:82:00.0 bottleneck@0000:81:00.0->0000:82:00.0)")
	// A single device string must not contain the comma the exporter uses to
	// split multiple failed devices apart, otherwise it gets cut into two series.
	assert.NotContains(t, dev, ",")
	assert.Equal(t, "16.0 GT/s PCIe", curr)
	assert.Equal(t, "32.0 GT/s PCIe", spec)
}

func TestIBPCIETreeSpeedChecker_MultipleLinksDegraded(t *testing.T) {
	hw := buildHW("mlx5_3", "0000:c1:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:c0:01.0", ChildBDF: "0000:c1:00.0",
			CurSpeed:       "8.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
		{
			ParentBDF: "0000:bf:00.0", ChildBDF: "0000:c0:01.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, curr, spec := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_3": hw})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Equal(t, 2, strings.Count(dev, "bottleneck"))
	// Both currents and both caps joined by comma, order matches PCIETreeLinks order.
	assert.Equal(t, "8.0 GT/s PCIe,16.0 GT/s PCIe", curr)
	assert.Equal(t, "32.0 GT/s PCIe,32.0 GT/s PCIe", spec)
}

func TestIBPCIETreeSpeedChecker_MultiNIC_OneDegraded(t *testing.T) {
	good := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "32.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	bad := buildHW("mlx5_3", "0000:c1:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:c0:01.0", ChildBDF: "0000:c1:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "32.0 GT/s PCIe", ChildMaxSpeed: "32.0 GT/s PCIe",
		},
	})
	status, dev, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{
		"mlx5_0": good,
		"mlx5_3": bad,
	})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Contains(t, dev, "mlx5_3")
	assert.NotContains(t, dev, "mlx5_0")
}

func TestIBPCIETreeSpeedChecker_UnparseableMaxSkipped(t *testing.T) {
	hw := buildHW("mlx5_0", "0000:82:00.0", []collector.PCIETreeLink{
		{
			ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
			CurSpeed:       "16.0 GT/s PCIe",
			ParentMaxSpeed: "", ChildMaxSpeed: "Unknown",
		},
	})
	status, _, _, _ := runSpeedChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
}
