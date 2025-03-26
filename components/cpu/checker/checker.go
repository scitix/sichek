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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/config/cpu"
)

func NewCheckers(ctx context.Context, cfg *cpu.CPUConfig) ([]common.Checker, error) {
	checkers := make([]common.Checker, 0)
	checker, err := NewCPUPerfChecker()
	if err != nil {
		return nil, fmt.Errorf("create cpu perf checker failed: %v", err)
	}
	checkers = append(checkers, checker)

	for name, eventCfg := range cpuCfg.CPU.EventCheckers {
		eventChecker, err := NewEventChecker(eventCfg)
		if err != nil {
			return nil, fmt.Errorf("create event %s checker failed: %v", name, err)
		}
		checkers = append(checkers, eventChecker)
	}

	return checkers, nil
}
