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

package collector

import (
	"sort"
	"strconv"
)

// Upstream (rdma-env-pre) metric names this summary reads. Frozen by the exporter's
// contract.go; passthrough itself does not depend on this list (parse keeps everything
// with the prefix) -- these are only for building the human-readable digest.
const (
	metricKnobVerdict  = "rdma_env_pre_knob_verdict"
	metricHostComplian = "rdma_env_pre_host_compliance"
	metricHostCount    = "rdma_env_pre_host_knob_count"
	metricObserve      = "rdma_env_pre_observe"
	metricDesiredValue = "rdma_env_pre_knob_desired_value"
	metricObservedVal  = "rdma_env_pre_knob_observed_value"
	metricKnobInfo     = "rdma_env_pre_knob_info"
)

// BuildSummary folds the scraped series into the snapshot digest: one row per knob with
// its desired/observed value, plus host compliance, per-verdict counts, and observes.
// It reads labels only -- no judgment, no invented values.
func BuildSummary(byName map[string][]Series) Summary {
	s := Summary{VerdictCounts: map[string]int{}}

	// SeriesTotal counts everything passed through, not just the families read here.
	for _, series := range byName {
		s.SeriesTotal += len(series)
	}

	// Host compliance state.
	if hc := byName[metricHostComplian]; len(hc) > 0 {
		s.HostCompliance = hc[0].Labels["state"]
	}

	// Per-verdict counts.
	for _, series := range byName[metricHostCount] {
		if v := series.Labels["verdict"]; v != "" {
			s.VerdictCounts[v] = int(series.Value)
		}
	}

	// Value indexes keyed by (device,knob,fabric).
	numDesired := indexByKnob(byName[metricDesiredValue])
	numObserved := indexByKnob(byName[metricObservedVal])
	infoDesired, infoObserved := indexInfo(byName[metricKnobInfo])

	for _, series := range byName[metricKnobVerdict] {
		l := series.Labels
		key := l["device"] + "\x00" + l["knob"] + "\x00" + l["fabric"]
		kv := KnobView{
			Device:         l["device"],
			Knob:           l["knob"],
			Fabric:         l["fabric"],
			Verdict:        l["verdict"],
			Severity:       l["severity"],
			RebootRequired: l["reboot_required"] == "true",
		}
		// Numeric knob takes the value families; otherwise the info family.
		if d, ok := numDesired[key]; ok {
			kv.Desired = d
		} else if d, ok := infoDesired[key]; ok {
			kv.Desired = d
		}
		if o, ok := numObserved[key]; ok {
			kv.Observed = o
		} else if o, ok := infoObserved[key]; ok {
			kv.Observed = o
		}
		s.Knobs = append(s.Knobs, kv)
	}

	// Observe-only inventory (observed value, no desired).
	for _, series := range byName[metricObserve] {
		s.Observes = append(s.Observes, ObserveView{
			Device:   series.Labels["device"],
			Knob:     series.Labels["knob"],
			Observed: formatValue(series.Value),
		})
	}

	sort.Slice(s.Knobs, func(i, j int) bool {
		if s.Knobs[i].Device != s.Knobs[j].Device {
			return s.Knobs[i].Device < s.Knobs[j].Device
		}
		return s.Knobs[i].Knob < s.Knobs[j].Knob
	})
	sort.Slice(s.Observes, func(i, j int) bool {
		if s.Observes[i].Device != s.Observes[j].Device {
			return s.Observes[i].Device < s.Observes[j].Device
		}
		return s.Observes[i].Knob < s.Observes[j].Knob
	})

	return s
}

// indexByKnob maps (device,knob,fabric) -> formatted numeric value.
func indexByKnob(series []Series) map[string]string {
	m := make(map[string]string, len(series))
	for _, s := range series {
		key := s.Labels["device"] + "\x00" + s.Labels["knob"] + "\x00" + s.Labels["fabric"]
		m[key] = formatValue(s.Value)
	}
	return m
}

// indexInfo maps (device,knob,fabric) -> desired/observed string from knob_info labels.
func indexInfo(series []Series) (desired, observed map[string]string) {
	desired = make(map[string]string, len(series))
	observed = make(map[string]string, len(series))
	for _, s := range series {
		key := s.Labels["device"] + "\x00" + s.Labels["knob"] + "\x00" + s.Labels["fabric"]
		desired[key] = s.Labels["desired"]
		observed[key] = s.Labels["observed"]
	}
	return desired, observed
}

// formatValue renders a float as the shortest decimal that round-trips.
func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
