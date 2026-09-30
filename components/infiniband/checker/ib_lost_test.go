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
)

func runIBLost(t *testing.T, info *collector.InfinibandInfo, spec *config.InfinibandSpec) (status, detail string) {
	t.Helper()
	c, err := NewIBLostChecker(spec)
	assert.NoError(t, err)
	r, err := c.Check(context.Background(), info)
	assert.NoError(t, err)
	// metric identity must never change
	assert.Equal(t, config.CheckIBLost, r.Name)
	assert.Equal(t, "IBLost", r.ErrorName)
	assert.Equal(t, consts.LevelCritical, r.Level)
	return r.Status, r.Detail
}

func TestIBLost_PhantomPCIe(t *testing.T) {
	info := &collector.InfinibandInfo{
		IBPFDevs:        map[string]string{"mlx5_2": "eth2", "mlx5_3": "eth3", "mlx5_4": "eth4"},
		IBPCIDevs:       map[string]string{},
		IBLostPCIDevs:   map[string]string{"0000:65:01.0": "0x15b3:0x1021"},
		HCAPCINum:       3,
		IBCapablePCINum: 3,
	}
	spec := &config.InfinibandSpec{
		IBPFDevs: map[string]string{"mlx5_2": "eth2", "mlx5_3": "eth3", "mlx5_4": "eth4"},
	}
	status, detail := runIBLost(t, info, spec)
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.True(t, strings.Contains(detail, "0000:65:01.0"), "detail should name the BDF: %s", detail)
}

func TestIBLost_CountMismatch(t *testing.T) {
	info := &collector.InfinibandInfo{
		IBPFDevs:        map[string]string{"mlx5_2": "eth2", "mlx5_3": "eth3", "mlx5_4": "eth4"},
		IBPCIDevs:       map[string]string{},
		IBLostPCIDevs:   map[string]string{},
		HCAPCINum:       3,
		IBCapablePCINum: 4,
	}
	spec := &config.InfinibandSpec{
		IBPFDevs: map[string]string{"mlx5_2": "eth2", "mlx5_3": "eth3", "mlx5_4": "eth4"},
	}
	status, detail := runIBLost(t, info, spec)
	assert.Equal(t, consts.StatusAbnormal, status)
	assert.True(t, strings.Contains(detail, "HCAPCINum != IBCapablePCINum"), "detail: %s", detail)
}

func TestIBLost_Healthy(t *testing.T) {
	info := &collector.InfinibandInfo{
		IBPFDevs:        map[string]string{"mlx5_1": "eth1", "mlx5_2": "eth2", "mlx5_3": "eth3", "mlx5_4": "eth4"},
		IBPCIDevs:       map[string]string{},
		IBLostPCIDevs:   map[string]string{},
		HCAPCINum:       4,
		IBCapablePCINum: 4,
	}
	spec := &config.InfinibandSpec{
		IBPFDevs: map[string]string{"mlx5_1": "eth1", "mlx5_2": "eth2", "mlx5_3": "eth3", "mlx5_4": "eth4"},
	}
	status, _ := runIBLost(t, info, spec)
	assert.Equal(t, consts.StatusNormal, status)
}
