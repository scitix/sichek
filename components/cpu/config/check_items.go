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
	"clock-sync-service": {
		Name:        "clock-sync-service",
		Description: "Check if PTP or NTP clock sync service is running",
		Spec:        "Active",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "ClockSyncServiceNotRunning",
		Suggestion:  "Ensure ptp4l, chronyd, or ntpd service is running for clock synchronization",
	},
	"clock-sync-offset": {
		Name:        "clock-sync-offset",
		Description: "Check if clock sync offset is within acceptable range",
		Spec:        "Within threshold",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "ClockSyncOffsetHigh",
		Suggestion:  "Check PTP/NTP configuration; high clock offset may cause distributed training issues",
	},
	"cpu-mce-uncorrected": {
		Name:        "cpu-mce-uncorrected",
		Description: "Check for uncorrected Machine Check Exceptions",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "CPUMCEUncorrected",
		Suggestion:  "Uncorrected MCE detected; this indicates a serious hardware error. Schedule maintenance",
	},
	"cpu-mce-corrected": {
		Name:        "cpu-mce-corrected",
		Description: "Check for corrected Machine Check Exceptions",
		Spec:        "Below threshold",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "CPUMCECorrectedHigh",
		Suggestion:  "High corrected MCE count detected; monitor for increasing errors and schedule preventive maintenance",
	},
}
