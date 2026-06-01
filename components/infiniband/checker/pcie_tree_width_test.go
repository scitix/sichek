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

	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runWidthChecker(t *testing.T, hws map[string]collector.IBHardWareInfo) (string, string, string, string) {
	t.Helper()
	ck, err := NewIBPCIETreeWidthChecker(&config.InfinibandSpec{})
	require.NoError(t, err)
	info := &collector.InfinibandInfo{IBHardWareInfo: hws}
	res, err := ck.Check(context.Background(), info)
	require.NoError(t, err)
	return res.Status, res.Device, res.Curr, res.Spec
}

func TestIBPCIETreeWidthChecker_NoLinksNormal(t *testing.T) {
	status, dev, _, _ := runWidthChecker(t, map[string]collector.IBHardWareInfo{
		"mlx5_0": {IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0", PCIETreeLinks: nil},
	})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
}

func TestIBPCIETreeWidthChecker_FullWidthNormal(t *testing.T) {
	hw := collector.IBHardWareInfo{
		IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0",
		PCIETreeLinks: []collector.PCIETreeLink{
			{
				ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
				CurWidth:       "16",
				ParentMaxWidth: "16", ChildMaxWidth: "16",
			},
		},
	}
	status, dev, _, _ := runWidthChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
	assert.Empty(t, dev)
}

func TestIBPCIETreeWidthChecker_OneLinkDegraded(t *testing.T) {
	hw := collector.IBHardWareInfo{
		IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0",
		PCIETreeLinks: []collector.PCIETreeLink{
			{
				ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
				CurWidth:       "8",
				ParentMaxWidth: "16", ChildMaxWidth: "16",
			},
		},
	}
	status, dev, curr, spec := runWidthChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.Contains(t, dev, "mlx5_0(0000:82:00.0, bottleneck@0000:80:01.0->0000:82:00.0)")
	assert.Equal(t, "8", curr)
	assert.Equal(t, "16", spec)
}

func TestIBPCIETreeWidthChecker_ParentSlotWiderThanChild_NoAlarm(t *testing.T) {
	// E.g. slot is x16 but the NIC supports only x8; running at x8 is correct.
	hw := collector.IBHardWareInfo{
		IBDev: "mlx5_0", PCIEBDF: "0000:82:00.0",
		PCIETreeLinks: []collector.PCIETreeLink{
			{
				ParentBDF: "0000:80:01.0", ChildBDF: "0000:82:00.0",
				CurWidth:       "8",
				ParentMaxWidth: "16", ChildMaxWidth: "8",
			},
		},
	}
	status, _, _, _ := runWidthChecker(t, map[string]collector.IBHardWareInfo{"mlx5_0": hw})
	assert.Equal(t, consts.StatusNormal, status)
}
