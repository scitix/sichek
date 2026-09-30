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

import "testing"

func TestParseOFEDVersion(t *testing.T) {
	tests := []struct {
		input       string
		wantMajor   string
		wantMinor   string
		shouldError bool
	}{
		// standard MLNX format
		{"MLNX_OFED_LINUX-5.9-0.5.6.0", "5.9", "0.5.6.0", false},
		{"MLNX_OFED_LINUX-24.10-2.1.8.0", "24.10", "2.1.8.0", false},

		// internal OFED format
		{"OFED-internal-23.10-1.1.9", "23.10", "1.1.9", false},
		{"OFED-internal-24.01-2.0.0", "24.01", "2.0.0", false},

		// error format
		{"MLNX_OFED_LINUX-5.9", "", "", true},
		{"OFED-internal-23.10", "", "", true},
		{"random-string", "", "", true},
	}

	for _, tt := range tests {
		major, minor, err := parseOFEDVersion(tt.input)
		if tt.shouldError {
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for input %q: %v", tt.input, err)
		}
		if major != tt.wantMajor || minor != tt.wantMinor {
			t.Errorf("parseOFEDVersion(%q) = (%s, %s), want (%s, %s)",
				tt.input, major, minor, tt.wantMajor, tt.wantMinor)
		}
	}
}

func TestCheckOFEDFormat(t *testing.T) {
	tests := []struct {
		spec     string
		curr     string
		expected bool
	}{
		{"MLNX_OFED_LINUX-5.9-0.5.6.0", "MLNX_OFED_LINUX-5.9-0.5.6.1", true},
		{"OFED-internal-23.10-1.1.9", "OFED-internal-23.10-1.2.0", true},
		{"OFED-internal-24.01-2.0.0", "MLNX_OFED_LINUX-5.8-1.0.0.1", true},
		{"bad-format", "MLNX_OFED_LINUX-5.9-0.5.6.0", false},
	}

	for _, tt := range tests {
		ok, _ := checkOFEDFormat(tt.spec, tt.curr)
		if ok != tt.expected {
			t.Errorf("checkOFEDFormat(%s, %s) = %v, want %v", tt.spec, tt.curr, ok, tt.expected)
		}
	}
}

func TestCheckOFEDVersion(t *testing.T) {
	tests := []struct {
		spec     string
		curr     string
		expected bool
	}{
		{"==MLNX_OFED_LINUX-5.9-0.5.6.0", "MLNX_OFED_LINUX-5.9-0.5.6.0", true},
		{">=MLNX_OFED_LINUX-5.8-0.5.6.0", "MLNX_OFED_LINUX-5.9-0.5.6.0", true},
		{"==OFED-internal-23.10-1.1.9", "OFED-internal-23.10-1.1.9", true},
		{"==OFED-internal-23.10-1.1.9", "OFED-internal-23.10-1.2.0", false},
		{"OFED-internal-23.10-1.1.9", "bad-format", false},
	}

	for _, tt := range tests {
		ok, _ := checkOFEDVersion(tt.spec, tt.curr)
		if ok != tt.expected {
			t.Errorf("checkOFEDVersion(%s, %s) = %v, want %v", tt.spec, tt.curr, ok, tt.expected)
		}
	}
}
