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

// CPUCheckItems is a map of check items for System
var CPUCheckItems = map[string]common.CheckerResult{
	"cpu-performance": {
		Name:        "cpu-performance",
		Description: "Check if all cpus are in performance mode",
		Spec:        "Enabled",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "CPUPerfModeNotEnabled",
		Suggestion:  "run `echo performance > /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor` to set all cpus to performance mode. Ideally this will be done automatically online",
	},
}
