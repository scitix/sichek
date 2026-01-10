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
	dependence "github.com/scitix/sichek/components/nvidia/checker/check_dependences"
	sram "github.com/scitix/sichek/components/nvidia/checker/check_ecc_sram"
	remap "github.com/scitix/sichek/components/nvidia/checker/check_remmaped_rows"
	"github.com/scitix/sichek/components/nvidia/config"
	nvutils "github.com/scitix/sichek/components/nvidia/utils" // Added import

	"github.com/sirupsen/logrus"
)

func NewCheckers(nvidiaCfg *config.NvidiaUserConfig, nvidiaSpecCfg *config.NvidiaSpec) ([]common.Checker, error) {
	checkerConstructors := map[string]func(*config.NvidiaSpec) (common.Checker, error){
		config.PCIeACSCheckerName:                   dependence.NewPCIeACSChecker,
		config.IOMMUCheckerName:                     dependence.NewIOMMUChecker,
		config.NVFabricManagerCheckerName:           dependence.NewNVFabricManagerChecker,
		config.NvPeerMemCheckerName:                 dependence.NewNvPeerMemChecker,
		config.IBGDACheckerName:                     NewIBGDAChecker,
		config.P2PCheckerName:                       NewP2PChecker,
		config.PCIeCheckerName:                      NewPCIeChecker,
		config.HardwareCheckerName:                  NewHardwareChecker,
		config.SoftwareCheckerName:                  NewSoftwareChecker,
		config.GpuPersistencedCheckerName:           NewGpuPersistenceChecker,
		config.GpuPStateCheckerName:                 NewGpuPStateChecker,
		config.NvlinkCheckerName:                    NewNvlinkChecker,
		config.AppClocksCheckerName:                 NewAppClocksChecker,
		config.ClockEventsCheckerName:               NewClockEventsChecker,
		config.SRAMAggUncorrectableCheckerName:      sram.NewSRAMAggUncorrectableChecker,
		config.SRAMHighcorrectableCheckerName:       sram.NewSRAMHighcorrectableChecker,
		config.SRAMVolatileUncorrectableCheckerName: sram.NewSRAMVolatileUncorrectableChecker,
		config.RemmapedRowsFailureCheckerName:       remap.NewRemmapedRowsFailureChecker,
		config.RemmapedRowsUncorrectableCheckerName: remap.NewRemmapedRowsUncorrectableChecker,
		config.RemmapedRowsPendingCheckerName:       remap.NewRemmapedRowsPendingChecker,
	}

	ignoredSet := make(map[string]struct{})
	for _, checker := range nvidiaCfg.Nvidia.IgnoredCheckers {
		ignoredSet[checker] = struct{}{}
	}

	// Check Compute Capability for IBGDA Logic
	shouldCheckIBGDA := false
	major, _, err := nvutils.GetComputeCapability(0)
	if err != nil {
		logrus.WithField("component", "NVIDIA-Checker").Warnf("Failed to get compute capability for IBGDA check: %v", err)
	} else {
		// Only check IBGDA if major is between 9 (inclusive) and 11 (exclusive)
		shouldCheckIBGDA = major >= 9 && major < 11
	}

	usedCheckersName := make([]string, 0)
	usedCheckers := make([]common.Checker, 0)
	cfg := nvidiaSpecCfg

	for checkerName := range config.GPUCheckItems {
		if _, found := ignoredSet[checkerName]; found {
			continue
		}

		// Skip IBGDA checker if compute capability requirements are not met
		if checkerName == config.IBGDACheckerName && !shouldCheckIBGDA {
			logrus.WithField("component", "NVIDIA-Checker").Infof("Skipping %s (Compute Capability major version %d not in [9, 11))", config.IBGDACheckerName, major)
			continue
		}

		if constructor, exists := checkerConstructors[checkerName]; exists {
			checker, err := constructor(cfg)
			if err != nil {
				logrus.WithError(err).WithField("checker", checkerName).Error("Failed to create checker")
				continue
			}
			usedCheckers = append(usedCheckers, checker)
			usedCheckersName = append(usedCheckersName, checkerName)
		}
	}
	logrus.WithField("component", "NVIDIA-Checker").Infof("usedCheckers: %v, ignoredCheckers: %v", usedCheckersName, nvidiaCfg.Nvidia.IgnoredCheckers)
	return usedCheckers, nil
}
