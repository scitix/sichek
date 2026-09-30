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
	"github.com/scitix/sichek/components/ovs/config"
)

func NewCheckers(cfg *config.OVSUserConfig, spec *config.OVSSpec) ([]common.Checker, error) {
	all := []common.Checker{
		&ServiceChecker{},
		&VersionChecker{spec: spec},
		&OtherConfigChecker{spec: spec},
		&BridgeChecker{spec: spec},
	}
	ignored := map[string]bool{}
	if cfg != nil && cfg.OVS != nil {
		for _, v := range cfg.OVS.IgnoredCheckers {
			ignored[v] = true
		}
	}
	var active []common.Checker
	for _, chk := range all {
		if !ignored[chk.Name()] {
			active = append(active, chk)
		}
	}
	return active, nil
}
