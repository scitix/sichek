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
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/sirupsen/logrus"
)

func NewCheckers(cfg *config.InfinibandUserConfig, spec *config.InfinibandSpec) ([]common.Checker, error) {

	// All checkers are registered unconditionally. Checkers that only apply to
	// a subset of hardware self-gate at Check() time (e.g. RoCEChecker skips
	// non-Ethernet devices). The link-layer of each HCA is only known after the
	// collector runs, which happens during HealthCheck — long after this
	// constructor — so gating registration on collected data here would never
	// match and silently drop the checker.
	checkerConstructors := map[string]func(*config.InfinibandSpec) (common.Checker, error){
		config.CheckIBOFED:      NewIBOFEDChecker,
		config.CheckIBFW:        NewFirmwareChecker,
		config.CheckIBState:     NewIBStateChecker,
		config.CheckIBPhyState:  NewIBPhyStateChecker,
		config.CheckIBPortSpeed: NewIBPortSpeedChecker,
		config.CheckPCIEMRR:     NewPCIEMRRChecker,
		config.CheckPCIESpeed:   NewIBPCIESpeedChecker,
		config.CheckPCIEWidth:   NewIBPCIEWidthChecker,
		config.CheckIBKmod:      NewIBKmodChecker,
		config.CheckIBDevs:      NewIBDevsChecker,
		config.CheckIBDriver:    NewIBDriverChecker,
		config.CheckIBLost:      NewIBLostChecker,
		config.CheckIBRailCount: NewIBRailCountChecker,
		config.CheckRoCE:        NewRoCEChecker,
		config.CheckPCIETreeSpeed: NewIBPCIETreeSpeedChecker,
		config.CheckPCIETreeWidth: NewIBPCIETreeWidthChecker,
		// config.CheckIBNUM:         dependence.NewIOMMUChecker,
		// config.CheckNetOperstate:  NewNetOperstateChecker,
		// config.CheckPCIEACS:       NewPCIEACSChecker,
	}

	ignoredSet := make(map[string]struct{})
	for _, checker := range cfg.Infiniband.IgnoredCheckers {
		ignoredSet[checker] = struct{}{}
	}
	usedCheckersName := make([]string, 0)
	usedCheckers := make([]common.Checker, 0)
	for checkerName := range config.InfinibandCheckItems {
		if _, found := ignoredSet[checkerName]; found {
			continue
		}

		if constructor, exists := checkerConstructors[checkerName]; exists {
			checker, err := constructor(spec)
			if err != nil {
				logrus.WithError(err).WithField("checker", checkerName).Error("Failed to create checker")
				continue
			}
			usedCheckers = append(usedCheckers, checker)
			usedCheckersName = append(usedCheckersName, checkerName)
		}
	}
	logrus.WithField("component", "Infiniband-Checker").Infof("usedCheckersName: %v, ignoredCheckers: %v", usedCheckersName, cfg.Infiniband.IgnoredCheckers)

	return usedCheckers, nil
}
