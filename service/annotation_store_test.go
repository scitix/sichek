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
package service

import (
	"testing"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// abnormalResult builds a Result with a single abnormal checker for the given item.
func abnormalResult(item, level, checkerName, errorName, device string) *common.Result {
	return &common.Result{
		Item:   item,
		Status: consts.StatusAbnormal,
		Level:  level,
		Checkers: []*common.CheckerResult{
			{
				Name:      checkerName,
				Status:    consts.StatusAbnormal,
				Level:     level,
				ErrorName: errorName,
				Device:    device,
			},
		},
	}
}

// errorNames collects the error names recorded for an item at a given level.
func errorNames(annos map[string][]*annotation, level string) []string {
	out := make([]string, 0)
	for _, a := range annos[level] {
		out = append(out, a.ErrorName)
	}
	return out
}

func TestAnnotationStore_SetReplacesItem(t *testing.T) {
	s := NewAnnotationStore()

	_, err := s.Apply(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "CPUCheck", "ErrA", "cpu0"))
	require.NoError(t, err)

	// A second (non-timeout) result for the same item replaces the previous issues.
	anno, err := s.Apply(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "CPUCheck", "ErrB", "cpu1"))
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"ErrB"}, errorNames(anno.CPU, consts.LevelCritical))
	assert.NotContains(t, errorNames(anno.CPU, consts.LevelCritical), "ErrA")
}

func TestAnnotationStore_AccumulatesAcrossComponents(t *testing.T) {
	s := NewAnnotationStore()

	_, err := s.Apply(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "CPUCheck", "ErrCPU", "cpu0"))
	require.NoError(t, err)
	anno, err := s.Apply(abnormalResult(consts.ComponentNameNvidia, consts.LevelFatal, "GpuCheck", "ErrGpu", "gpu0"))
	require.NoError(t, err)

	// Both components' issues coexist in the accumulated annotation.
	assert.ElementsMatch(t, []string{"ErrCPU"}, errorNames(anno.CPU, consts.LevelCritical))
	assert.ElementsMatch(t, []string{"ErrGpu"}, errorNames(anno.NVIDIA, consts.LevelFatal))
}

func TestAnnotationStore_TimeoutAppendsWithoutWiping(t *testing.T) {
	s := NewAnnotationStore()

	// Real issue recorded first.
	_, err := s.Apply(abnormalResult(consts.ComponentNameNvidia, consts.LevelFatal, "GpuCheck", "ErrGpu", "gpu0"))
	require.NoError(t, err)

	// A HealthCheckTimeout result for the same item must append, not replace.
	anno, err := s.Apply(abnormalResult(consts.ComponentNameNvidia, consts.LevelCritical, "HealthCheckTimeout", "Timeout", ""))
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"ErrGpu"}, errorNames(anno.NVIDIA, consts.LevelFatal))
	assert.ElementsMatch(t, []string{"Timeout"}, errorNames(anno.NVIDIA, consts.LevelCritical))
}

func TestAnnotationStore_DedupSameError(t *testing.T) {
	s := NewAnnotationStore()

	// Two timeout results with the same error/level should dedup to one entry.
	_, err := s.Apply(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "HealthCheckTimeout", "Timeout", ""))
	require.NoError(t, err)
	anno, err := s.Apply(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "HealthCheckTimeout", "Timeout", ""))
	require.NoError(t, err)

	assert.Len(t, anno.CPU[consts.LevelCritical], 1)
}

func TestAnnotationStore_ReturnsDeepCopy(t *testing.T) {
	s := NewAnnotationStore()

	anno, err := s.Apply(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "CPUCheck", "ErrA", "cpu0"))
	require.NoError(t, err)

	// Mutating the returned copy must not affect the store's internal state.
	anno.CPU = nil
	next, err := s.Apply(abnormalResult(consts.ComponentNameNvidia, consts.LevelFatal, "GpuCheck", "ErrGpu", "gpu0"))
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ErrA"}, errorNames(next.CPU, consts.LevelCritical))
}

func TestAnnotationStore_UnsupportedItemTimeoutDoesNotPanic(t *testing.T) {
	s := NewAnnotationStore()

	// transceiver is not in getAnnotationsByItem's switch; an append (timeout) on it
	// surfaces an error but must not panic, and must still return a usable annotation.
	anno, err := s.Apply(abnormalResult(consts.ComponentNameTransceiver, consts.LevelWarning, "HealthCheckTimeout", "Timeout", ""))
	assert.Error(t, err)
	assert.NotNil(t, anno)
}

func TestIsHealthCheckTimeout(t *testing.T) {
	assert.True(t, isHealthCheckTimeout(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "HealthCheckTimeout", "Timeout", "")))
	// Normal status with the timeout name is not treated as a timeout-append.
	normal := abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "HealthCheckTimeout", "Timeout", "")
	normal.Status = consts.StatusNormal
	assert.False(t, isHealthCheckTimeout(normal))
	// A regular checker is not a timeout.
	assert.False(t, isHealthCheckTimeout(abnormalResult(consts.ComponentNameCPU, consts.LevelCritical, "CPUCheck", "ErrA", "cpu0")))
	assert.False(t, isHealthCheckTimeout(nil))
}
