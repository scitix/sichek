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
	"time"

	"github.com/scitix/sichek/components/gpuevents/config"
	"github.com/sirupsen/logrus"
)

// IndicatorState represents the real-time status of a specific gpuevents indicator,
// including whether the condition is currently met and how long it's been active.
type IndicatorState struct {
	Active   bool          // Whether the indicator currently meets the gpuevents condition
	Value    int64         // The current value of the indicator
	Duration time.Duration // Accumulated duration during which the condition is met
}

// IndicatorStates tracks the status of all indicators for a single device.
type IndicatorStates struct {
	Indicators map[string]*IndicatorState
	LastUpdate time.Time // Last update timestamp for this device's indicators
}

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
}

func GetIndicatorDuration(indicatorName string, infoValue int64, spec *config.GpuEventRule, now time.Time, lastUpdate time.Time) int64 {
	var res int64 = 0
	indicator := spec.Indicators[indicatorName]
	if indicator == nil {
		logrus.WithField("collector", "gpuevents").Errorf("failed to get indicator spec of %s", indicatorName)
		return res
	}

	if (infoValue < indicator.Threshold && indicator.CompareType == string(config.CompareLow)) ||
		(infoValue > indicator.Threshold && indicator.CompareType == string(config.CompareHigh)) ||
		(infoValue == indicator.Threshold && indicator.CompareType == string(config.CompareEqual)) {
		if !lastUpdate.IsZero() {
			res = int64(now.Sub(lastUpdate).Seconds())
		}
	}
	return res
}
