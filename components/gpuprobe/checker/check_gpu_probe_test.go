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

	"github.com/scitix/sichek/components/gpuprobe/collector"
	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkChecker(t *testing.T, threshold int) *GpuProbeChecker {
	spec := config.DefaultSpec()
	spec.FailConsecutiveThreshold = threshold
	ch, err := NewGpuProbeChecker(spec)
	require.NoError(t, err)
	return ch.(*GpuProbeChecker)
}

func info(results ...collector.GpuProbeResult) *collector.GpuProbeInfo {
	return &collector.GpuProbeInfo{PerGPU: results}
}

func TestCheck_PassIsNormal(t *testing.T) {
	c := mkChecker(t, 1)
	r, err := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", Outcome: collector.OutcomePass}))
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, r.Status)
}

func TestCheck_FailImmediateCriticalWhenThreshold1(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 6, BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelCritical, r.Level)
	assert.Equal(t, "GPUProbeFailed", r.ErrorName)
}

func TestCheck_FailDebouncedWhenThreshold2(t *testing.T) {
	c := mkChecker(t, 2)
	r1, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 6, BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.LevelWarning, r1.Level)
	r2, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 6, BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.LevelCritical, r2.Level)
}

func TestCheck_PassResetsConsecutive(t *testing.T) {
	c := mkChecker(t, 2)
	c.Check(context.Background(), info(collector.GpuProbeResult{BDF: "bb:00", Outcome: collector.OutcomeFail}))
	c.Check(context.Background(), info(collector.GpuProbeResult{BDF: "bb:00", Outcome: collector.OutcomePass}))
	r, _ := c.Check(context.Background(), info(collector.GpuProbeResult{BDF: "bb:00", Outcome: collector.OutcomeFail}))
	assert.Equal(t, consts.LevelWarning, r.Level)
}

func TestCheck_EnvErrIsWarning(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", Outcome: collector.OutcomeEnvErr}))
	assert.Equal(t, consts.StatusAbnormal, r.Status)
	assert.Equal(t, consts.LevelWarning, r.Level)
}

func TestCheck_TimeoutIsCritical(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", Outcome: collector.OutcomeTimeout}))
	assert.Equal(t, consts.LevelCritical, r.Level)
}

func TestCheck_AllBusyIsNormal(t *testing.T) {
	c := mkChecker(t, 1)
	r, _ := c.Check(context.Background(), info(
		collector.GpuProbeResult{Index: 0, BDF: "bb:00", State: "busy", Outcome: collector.OutcomeSkip}))
	assert.Equal(t, consts.StatusNormal, r.Status)
}
