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
	"github.com/scitix/sichek/config/nvidia"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sirupsen/logrus"
)

func NewCheckers(nvidiaCfg *config.NvidiaConfig, nvmlInst nvml.Interface) ([]common.Checker, error) {
	checkerConstructors := map[string]func(*nvidia.NvidiaSpecItem) (common.Checker, error){
		nvidia.PCIeACSCheckerName:         dependence.NewPCIeACSChecker,
		nvidia.IOMMUCheckerName:           dependence.NewIOMMUChecker,
		nvidia.NVFabricManagerCheckerName: dependence.NewNVFabricManagerChecker,
		nvidia.NvPeerMemCheckerName:       dependence.NewNvPeerMemChecker,
		nvidia.PCIeCheckerName:            NewPCIeChecker,
		// config.HardwareCheckerName:                  NewHardwareChecker,
		nvidia.SoftwareCheckerName:                  NewSoftwareChecker,
		nvidia.GpuPersistenceCheckerName:            NewGpuPersistenceChecker,
		nvidia.GpuPStateCheckerName:                 NewGpuPStateChecker,
		nvidia.NvlinkCheckerName:                    NewNvlinkChecker,
		nvidia.AppClocksCheckerName:                 NewAppClocksChecker,
		nvidia.ClockEventsCheckerName:               NewClockEventsChecker,
		nvidia.SRAMAggUncorrectableCheckerName:      sram.NewSRAMAggUncorrectableChecker,
		nvidia.SRAMHighcorrectableCheckerName:       sram.NewSRAMHighcorrectableChecker,
		nvidia.SRAMVolatileUncorrectableCheckerName: sram.NewSRAMVolatileUncorrectableChecker,
		nvidia.RemmapedRowsFailureCheckerName:       remap.NewRemmapedRowsFailureChecker,
		nvidia.RemmapedRowsUncorrectableCheckerName: remap.NewRemmapedRowsUncorrectableChecker,
		nvidia.RemmapedRowsPendingCheckerName:       remap.NewRemmapedRowsPendingChecker,
	}

	ignoredSet := make(map[string]struct{})
	for _, checker := range nvidiaCfg.ComponentConfig.Nvidia.IgnoredCheckers {
		ignoredSet[checker] = struct{}{}
	}

	usedCheckersName := make([]string, 0)
	usedCheckers := make([]common.Checker, 0)
	cfg := &nvidiaCfg.Spec

	for checkerName := range nvidia.GPUCheckItems {
		if _, found := ignoredSet[checkerName]; found {
			continue
		}

		if checkerName == nvidia.HardwareCheckerName {
			checker, err := NewHardwareChecker(cfg, nvmlInst)
			if err != nil {
				logrus.WithError(err).WithField("checker", checkerName).Error("Failed to create checker")
				continue
			}
			usedCheckers = append(usedCheckers, checker)
			usedCheckersName = append(usedCheckersName, checkerName)
			usedCheckersName = append(usedCheckersName, checkerName)
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
			usedCheckersName = append(usedCheckersName, checkerName)
		}
	}
	logrus.WithField("component", "NVIDIA-Checker").Infof("usedCheckers: %v, ignoredCheckers: %v", usedCheckers, nvidiaCfg.ComponentConfig.Nvidia.IgnoredCheckers)
	logrus.WithField("component", "NVIDIA-Checker").Infof("usedCheckers: %v, ignoredCheckers: %v", usedCheckers, nvidiaCfg.ComponentConfig.Nvidia.IgnoredCheckers)
	return usedCheckers, nil
}
