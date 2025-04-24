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
package topotest

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

const (
	PciTopoNumaCheckerName   = "PciTopoNumaCheckerName"
	PciTopoSwitchCheckerName = "PciTopoSwitchCheckerName"
)

// PciTopoCheckItems is a map of check items for Topo
var PciTopoCheckItems = map[string]common.CheckerResult{
	PciTopoNumaCheckerName: {
		Name:        PciTopoNumaCheckerName,
		Description: "",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "Numa-Device RelationError",
		Suggestion:  "Check Device Topo",
	},
	PciTopoSwitchCheckerName: {
		Name:        PciTopoSwitchCheckerName,
		Description: "",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "Switch-Device RelationError",
		Suggestion:  "Check Device Topo",
	},
}
