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
	"github.com/scitix/sichek/components/transceiver/config"
)

// NewCheckers creates all transceiver checkers, filtering out any in the ignored list.
func NewCheckers(cfg *config.TransceiverUserConfig, spec *config.TransceiverSpec) ([]common.Checker, error) {
	checkers := []common.Checker{
		&TxPowerChecker{spec: spec},
		&RxPowerChecker{spec: spec},
		&TemperatureChecker{spec: spec},
		&VoltageChecker{spec: spec},
		&BiasCurrentChecker{spec: spec},
		&VendorChecker{spec: spec},
		&LinkErrorsChecker{
			spec:      spec,
			prevErrors: make(map[string]map[string]uint64),
		},
		&PresenceChecker{spec: spec},
	}

	ignoredMap := make(map[string]bool)
	if cfg != nil && cfg.Transceiver != nil {
		for _, v := range cfg.Transceiver.IgnoredCheckers {
			ignoredMap[v] = true
		}
	}

	var active []common.Checker
	for _, chk := range checkers {
		if !ignoredMap[chk.Name()] {
			active = append(active, chk)
		}
	}
	return active, nil
}
