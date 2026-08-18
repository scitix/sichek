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

package metrics

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/scitix/sichek/components/transceiver/collector"
)

// TestStructToMetricsMap_TransceiverCounters pins the reflection-driven export of
// the new mlxlink -c physical counters / BER fields on ModuleInfo. TransceiverMetrics
// exports these gauges as sichek_transceiver_module_<json-tag> purely by walking
// json tags, so a rename of any tag would silently rename (or drop) the metric —
// this asserts the exact names and values a populated module produces.
func TestStructToMetricsMap_TransceiverCounters(t *testing.T) {
	mod := collector.ModuleInfo{
		RawPhysicalBER:           2e-9,
		EffectivePhysicalBER:     1e-12,
		EffectivePhysicalErrors:  42,
		LinkDownCounter:          3,
		LinkErrorRecoveryCounter: 7,
		TimeSinceLastClearMin:    120.5,
	}

	got := map[string]*StructMetrics{}
	StructToMetricsMap(reflect.ValueOf(mod), "", "json", got)

	// metric-name (json tag) → expected gauge value
	want := map[string]float64{
		"raw_physical_ber":            2e-9,
		"effective_physical_ber":      1e-12,
		"effective_physical_errors":   42,
		"link_down_counter":           3,
		"link_error_recovery_counter": 7,
		"time_since_last_clear_min":   120.5,
	}

	for name, exp := range want {
		sm, ok := got[name]
		if assert.Truef(t, ok, "metric %q must be exported from ModuleInfo json tags", name) {
			assert.InDelta(t, exp, sm.MetricsValue, exp*1e-6+1e-300)
		}
	}
}
