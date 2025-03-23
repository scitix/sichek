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

func NewCheckers(cfg *config.InfinibandConfig, spec *config.InfinibandSpec) ([]common.Checker, error) {
	checkerConstructors := map[string]func(*config.InfinibandSpec) (common.Checker, error){
		config.ChekIBOFED: NewIBOFEDChecker,
		// config.ChekIBNUM:           dependence.NewIOMMUChecker,
		config.ChekIBFW:         NewFirmwareChecker,
		config.ChekIBState:      NewIBStateChecker,
		config.ChekIBPhyState:   NewIBPhyStateChecker,
		config.ChekIBPortSpeed:  NewIBPortSpeedChecker,
		// config.ChekNetOperstate: NewNetOperstateChecker,
		// config.CheckPCIEACS:       NewPCIEACSChecker,
		config.CheckPCIEMRR:   NewPCIEMRRChecker,
		config.CheckPCIESpeed: NewIBPCIESpeedChecker,
		config.CheckPCIEWidth: NewIBPCIEWidthChecker,
		// config.CheckPCIETreeSpeed: NewBPCIETreeSpeedChecker,
		// config.CheckPCIETreeWidth: NewIBPCIETreeWidthChecker,
		config.CheckIBKmod: NewIBKmodChecker,
		config.CheckIBDevs: NewIBDevsChecker,
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
