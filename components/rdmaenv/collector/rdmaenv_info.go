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
	"time"

	"github.com/scitix/sichek/components/common"
)

// Series is one Prometheus sample, carried verbatim (name + labels + value) with no
// interpretation of its meaning. It is what the metrics layer re-exports.
type Series struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

// KnobView merges the same knob across the rdma_env_pre_knob_verdict /
// _knob_desired_value / _knob_observed_value / _knob_info families into one row. It is
// assembled by reading labels only -- no judgment.
type KnobView struct {
	Device         string `json:"device"`
	Knob           string `json:"knob"`
	Fabric         string `json:"fabric"`
	Verdict        string `json:"verdict"`
	Severity       string `json:"severity"`
	Desired        string `json:"desired"`
	Observed       string `json:"observed"`
	RebootRequired bool   `json:"reboot_required"`
}

// ObserveView is a read-only inventory item: observed value only, no desired.
type ObserveView struct {
	Device   string `json:"device"`
	Knob     string `json:"knob"`
	Observed string `json:"observed"`
}

// Summary is the human-readable digest that lands in the snapshot.
type Summary struct {
	HostCompliance string         `json:"host_compliance"`
	VerdictCounts  map[string]int `json:"verdict_counts"`
	SeriesTotal    int            `json:"series_total"`
	Knobs          []KnobView     `json:"knobs"`
	Observes       []ObserveView  `json:"observes,omitempty"`
}

// Info is the collector output. Series is kept in memory for the metrics layer only
// (json:"-"), so the snapshot carries just Summary + meta.
type Info struct {
	Available bool                `json:"available"`
	Endpoint  string              `json:"endpoint"`
	ScrapedAt time.Time           `json:"scraped_at"`
	Error     string              `json:"error,omitempty"`
	Series    map[string][]Series `json:"-"`
	Summary   Summary             `json:"summary"`
}

// JSON implements common.Info.
func (i *Info) JSON() (string, error) {
	data, err := common.JSON(i)
	return string(data), err
}
