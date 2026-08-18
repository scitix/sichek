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
	"fmt"
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuprobe/collector"
	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/scitix/sichek/consts"
)

const GpuProbeCheckerName = config.GpuProbeCheckerName

type GpuProbeChecker struct {
	name string
	spec *config.GpuProbeSpec
	mu   sync.Mutex
	// consecutive fail/timeout count per GPU (keyed by BDF), for debounce.
	consecFails map[string]int
}

func NewGpuProbeChecker(spec *config.GpuProbeSpec) (common.Checker, error) {
	return &GpuProbeChecker{
		name:        GpuProbeCheckerName,
		spec:        spec,
		consecFails: make(map[string]int),
	}, nil
}

func (c *GpuProbeChecker) Name() string { return c.name }

func (c *GpuProbeChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.GpuProbeInfo)
	if !ok {
		return nil, fmt.Errorf("invalid gpuprobe info type")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	result := config.GpuProbeCheckItems[GpuProbeCheckerName]
	result.Status = consts.StatusNormal
	result.Level = consts.LevelInfo

	var hardBad, softBad, warn, tested, skipped []string
	for _, g := range info.PerGPU {
		dev := fmt.Sprintf("GPU%d %s", g.Index, g.BDF)
		switch g.Outcome {
		case collector.OutcomePass:
			c.consecFails[g.BDF] = 0
			tested = append(tested, dev)
		case collector.OutcomeSkip:
			c.consecFails[g.BDF] = 0
			skipped = append(skipped, dev)
		case collector.OutcomeFail, collector.OutcomeTimeout:
			c.consecFails[g.BDF]++
			if c.consecFails[g.BDF] >= c.spec.FailConsecutiveThreshold {
				hardBad = append(hardBad, dev)
			} else {
				softBad = append(softBad, dev) // below debounce threshold: observe as Warning
			}
		case collector.OutcomeEnvErr, collector.OutcomeExecErr:
			warn = append(warn, dev)
		}
	}

	switch {
	case len(hardBad) > 0:
		result.Status = consts.StatusAbnormal
		result.Level = c.spec.FailLevel
		result.ErrorName = "GPUProbeFailed"
		result.Curr = fmt.Sprintf("%d bad", len(hardBad))
		result.Detail = fmt.Sprintf("GPU compute self-test FAILED/timed-out on idle GPUs: %v", hardBad)
	case len(softBad) > 0 || len(warn) > 0:
		result.Status = consts.StatusAbnormal
		result.Level = c.spec.EnvErrorLevel
		result.ErrorName = "GPUProbeEnvError"
		result.Detail = fmt.Sprintf("gpuprobe warnings: pending-fail=%v env/exec-err=%v", softBad, warn)
	default:
		result.Detail = fmt.Sprintf("gpuprobe OK: tested=%v skipped(busy)=%v", tested, skipped)
	}
	return &result, nil
}
