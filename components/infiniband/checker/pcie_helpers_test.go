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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPcieSpeedLessThan(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"less_simple", "16", "32", true},
		{"greater", "32", "16", false},
		{"equal_no_decimals", "16", "16", false},
		{"equal_with_decimals", "32.0", "32", false},
		{"less_gt_suffix", "16.0 GT/s PCIe", "32.0 GT/s PCIe", true},
		{"unparseable_a_returns_false", "abc", "32", false},
		{"unparseable_b_returns_false", "16", "xyz", false},
		{"empty_returns_false", "", "32", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, pcieSpeedLessThan(tc.a, tc.b))
		})
	}
}

func TestMinNumericSpeed(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want string
	}{
		{"a_smaller", "16.0 GT/s PCIe", "32.0 GT/s PCIe", "16.0 GT/s PCIe"},
		{"b_smaller", "32", "16", "16"},
		{"equal_returns_a", "32", "32", "32"},
		{"a_unparseable_returns_empty", "abc", "16", ""},
		{"b_unparseable_returns_empty", "16", "xyz", ""},
		{"both_empty_returns_empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, minNumericSpeed(tc.a, tc.b))
		})
	}
}
