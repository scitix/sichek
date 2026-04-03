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

// BusinessCheckItems defines the default CheckerResult template for business network transceivers.
var BusinessCheckItems = map[string]common.CheckerResult{
	TxPowerCheckerName: {
		Name:        TxPowerCheckerName,
		Description: "Check transceiver Tx optical power per lane against module alarm thresholds with margin",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "All Tx power levels are within acceptable range",
		ErrorName:   "TxPowerOutOfRange",
		Suggestion:  "Check fiber connections, clean fiber connectors, or replace transceiver module",
	},
	RxPowerCheckerName: {
		Name:        RxPowerCheckerName,
		Description: "Check transceiver Rx optical power per lane against module alarm thresholds with margin",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "All Rx power levels are within acceptable range",
		ErrorName:   "RxPowerOutOfRange",
		Suggestion:  "Check fiber connections, clean fiber connectors, inspect remote end transceiver",
	},
	TemperatureCheckerName: {
		Name:        TemperatureCheckerName,
		Description: "Check transceiver module temperature against warning and critical thresholds",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "All transceiver temperatures are within acceptable range",
		ErrorName:   "TransceiverOverheat",
		Suggestion:  "Check airflow and cooling, reduce ambient temperature, or replace overheating module",
	},
	VoltageCheckerName: {
		Name:        VoltageCheckerName,
		Description: "Check transceiver supply voltage against module built-in alarm thresholds",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "All transceiver voltages are within acceptable range",
		ErrorName:   "VoltageOutOfRange",
		Suggestion:  "Check power supply rails and transceiver seating, replace transceiver if issue persists",
	},
	BiasCurrentCheckerName: {
		Name:        BiasCurrentCheckerName,
		Description: "Check transceiver laser bias current per lane for abnormal values",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "All laser bias currents are normal",
		ErrorName:   "BiasCurrentAbnormal",
		Suggestion:  "Laser may be failing, replace the transceiver module",
	},
	VendorCheckerName: {
		Name:        VendorCheckerName,
		Description: "Check transceiver vendor is in the approved vendor list",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All transceiver vendors are approved",
		ErrorName:   "VendorNotApproved",
		Suggestion:  "Replace with an approved transceiver vendor module",
	},
	LinkErrorsCheckerName: {
		Name:        LinkErrorsCheckerName,
		Description: "Check transceiver link error counter delta between consecutive health checks",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "No link error increase detected",
		ErrorName:   "LinkErrorsIncreased",
		Suggestion:  "Check fiber integrity, clean connectors, replace transceiver or cable",
	},
	PresenceCheckerName: {
		Name:        PresenceCheckerName,
		Description: "Check all expected transceiver module slots are populated",
		Status:      consts.StatusNormal,
		Level:       consts.LevelFatal,
		Detail:      "All transceiver modules are present",
		ErrorName:   "TransceiverMissing",
		Suggestion:  "Re-seat or replace the missing transceiver module",
	},
}

// ManagementCheckItems defines the default CheckerResult template for management network transceivers.
// All levels are warning — management network issues are non-critical.
var ManagementCheckItems = map[string]common.CheckerResult{
	TxPowerCheckerName: {
		Name:        TxPowerCheckerName,
		Description: "Check transceiver Tx optical power per lane against module alarm thresholds with margin",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All Tx power levels are within acceptable range",
		ErrorName:   "TxPowerOutOfRange",
		Suggestion:  "Check fiber connections, clean fiber connectors, or replace transceiver module",
	},
	RxPowerCheckerName: {
		Name:        RxPowerCheckerName,
		Description: "Check transceiver Rx optical power per lane against module alarm thresholds with margin",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All Rx power levels are within acceptable range",
		ErrorName:   "RxPowerOutOfRange",
		Suggestion:  "Check fiber connections, clean fiber connectors, inspect remote end transceiver",
	},
	TemperatureCheckerName: {
		Name:        TemperatureCheckerName,
		Description: "Check transceiver module temperature against warning and critical thresholds",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All transceiver temperatures are within acceptable range",
		ErrorName:   "TransceiverOverheat",
		Suggestion:  "Check airflow and cooling, reduce ambient temperature, or replace overheating module",
	},
	VoltageCheckerName: {
		Name:        VoltageCheckerName,
		Description: "Check transceiver supply voltage against module built-in alarm thresholds",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All transceiver voltages are within acceptable range",
		ErrorName:   "VoltageOutOfRange",
		Suggestion:  "Check power supply rails and transceiver seating, replace transceiver if issue persists",
	},
	BiasCurrentCheckerName: {
		Name:        BiasCurrentCheckerName,
		Description: "Check transceiver laser bias current per lane for abnormal values",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All laser bias currents are normal",
		ErrorName:   "BiasCurrentAbnormal",
		Suggestion:  "Laser may be failing, replace the transceiver module",
	},
	VendorCheckerName: {
		Name:        VendorCheckerName,
		Description: "Check transceiver vendor is in the approved vendor list",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All transceiver vendors are approved",
		ErrorName:   "VendorNotApproved",
		Suggestion:  "Replace with an approved transceiver vendor module",
	},
	LinkErrorsCheckerName: {
		Name:        LinkErrorsCheckerName,
		Description: "Check transceiver link error counter delta between consecutive health checks",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "No link error increase detected",
		ErrorName:   "LinkErrorsIncreased",
		Suggestion:  "Check fiber integrity, clean connectors, replace transceiver or cable",
	},
	PresenceCheckerName: {
		Name:        PresenceCheckerName,
		Description: "Check all expected transceiver module slots are populated",
		Status:      consts.StatusNormal,
		Level:       consts.LevelWarning,
		Detail:      "All transceiver modules are present",
		ErrorName:   "TransceiverMissing",
		Suggestion:  "Re-seat or replace the missing transceiver module",
	},
}

// GetCheckItem returns the CheckerResult template for the given checker and network type.
func GetCheckItem(checkerName, networkType string) common.CheckerResult {
	if networkType == "management" {
		if item, ok := ManagementCheckItems[checkerName]; ok {
			return item
		}
	}
	if item, ok := BusinessCheckItems[checkerName]; ok {
		return item
	}
	return common.CheckerResult{Name: checkerName}
}
