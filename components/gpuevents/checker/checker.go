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
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuevents/config"
	"github.com/sirupsen/logrus"
)

func NewCheckers(cfg *config.GpuCostomEventsUserConfig, eventRules map[string]*config.GpuEventRule) ([]common.Checker, error) {
	checkerConstructors := map[string]func(*config.GpuCostomEventsUserConfig, *config.GpuEventRule) common.Checker{
		config.GPUHangCheckerName:       NewGpuHangChecker,
		config.SmClkStuckLowCheckerName: NewSmClkStuckLowChecker,
	}

	ignoredSet := make(map[string]struct{})
	for _, checker := range cfg.UserConfig.IgnoredCheckers {
		ignoredSet[checker] = struct{}{}
	}

	usedCheckersName := make([]string, 0)
	usedCheckers := make([]common.Checker, 0)

	for checkerName := range checkerConstructors {
		if _, found := ignoredSet[checkerName]; found {
			continue
		}

		if constructor, exists := checkerConstructors[checkerName]; exists {
			checker := constructor(cfg, eventRules[checkerName])
			usedCheckers = append(usedCheckers, checker)
			usedCheckersName = append(usedCheckersName, checkerName)
		}
	}
	logrus.WithField("component", "gpuevents").Infof("usedCheckers: %v, ignoredCheckers: %v", usedCheckersName, cfg.UserConfig.IgnoredCheckers)
	return usedCheckers, nil
}
