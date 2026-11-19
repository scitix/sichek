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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An IB device that disappears from sysfs mid-run (VF reclaim, hot-unplug)
// must degrade to empty fields, not panic the whole infiniband component.
const vanishedIBDev = "mlx5_sichek_test_vanished"

func TestGetIBDevBDF_VanishedDevice(t *testing.T) {
	assert.Empty(t, GetIBDevBDF(vanishedIBDev))
}

func TestGetVFNum_VanishedDevice(t *testing.T) {
	var hw IBHardWareInfo
	assert.NotPanics(t, func() {
		assert.Equal(t, "", hw.GetVFNum(vanishedIBDev))
	})
}

func TestIBHardWareInfoCollect_VanishedDevice(t *testing.T) {
	for _, role := range []string{"sriovNode", ""} {
		t.Run("role="+role, func(t *testing.T) {
			var hw IBHardWareInfo
			assert.NotPanics(t, func() {
				hw.Collect(context.Background(), vanishedIBDev, 1, role)
			})
			assert.Empty(t, hw.PCIEBDF)
			assert.Empty(t, hw.NumaNode)
			assert.Empty(t, hw.CPULists)
		})
	}
}
