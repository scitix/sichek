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
	"github.com/scitix/sichek/components/gpfs/config"

	"github.com/sirupsen/logrus"
)

func NewCheckers(cfg *config.GpfsUserConfig) ([]common.Checker, error) {
	ignoredSet := make(map[string]struct{})
	for _, checker := range cfg.Gpfs.IgnoredCheckers {
		ignoredSet[checker] = struct{}{}
	}
	usedCheckersName := make([]string, 0)
	usedCheckers := make([]common.Checker, 0)
	for checkerName := range config.GPFSCheckItems {
		if _, found := ignoredSet[checkerName]; found {
			continue
		}

		checker, err := NewXStorHealthChecker(checkerName)
		if err != nil {
			logrus.WithField("component", "gpfs").WithError(err).WithField("checker", checkerName).Error("Failed to create checker")
			continue
		}
		usedCheckers = append(usedCheckers, checker)
		usedCheckersName = append(usedCheckersName, checkerName)
	}
	logrus.WithField("component", "gpfs-checker").Infof("usedCheckers: %v, ignoredCheckers: %v", usedCheckersName, cfg.Gpfs.IgnoredCheckers)

	return usedCheckers, nil
}
