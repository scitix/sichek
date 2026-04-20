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
package config

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

var MemoryCheckItems = map[string]common.CheckerResult{
	"memory-ecc-uncorrected": {
		Name:        "memory-ecc-uncorrected",
		Description: "Check for uncorrectable memory ECC errors",
		Spec:        "0",
		Level:       consts.LevelCritical,
		ErrorName:   "MemoryECCUncorrected",
		Suggestion:  "Uncorrectable memory error detected. Identify faulty DIMM and replace",
	},
	"memory-ecc-corrected": {
		Name:        "memory-ecc-corrected",
		Description: "Check correctable memory ECC error count is below threshold",
		Spec:        "<100",
		Level:       consts.LevelWarning,
		ErrorName:   "MemoryECCCorrectedHigh",
		Suggestion:  "Correctable memory errors increasing. Monitor DIMM health and plan replacement",
	},
	"memory-capacity": {
		Name:        "memory-capacity",
		Description: "Check total memory matches expected specification",
		Spec:        "matches spec",
		Level:       consts.LevelCritical,
		ErrorName:   "MemoryCapacityMismatch",
		Suggestion:  "Memory capacity does not match spec. Check for failed DIMMs",
	},
}
