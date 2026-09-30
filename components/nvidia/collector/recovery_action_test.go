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

import "testing"

func TestParseRecoveryAction(t *testing.T) {
	// Real nvidia-smi -q sections from an H200 node (driver 570.86.15): a healthy
	// GPU reports "None", a faulted GPU stuck awaiting reset reports "Reset". The
	// deprecated "Reset Required" line must NOT be picked up.
	healthy := `    GPU Reset Status
        Reset Required                    : Requested functionality has been deprecated
        Drain and Reset Recommended       : Requested functionality has been deprecated
    GPU Recovery Action                   : None`
	faulted := `    GPU Reset Status
        Reset Required                    : Requested functionality has been deprecated
        Drain and Reset Recommended       : Requested functionality has been deprecated
    GPU Recovery Action                   : Reset`

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "healthy None", input: healthy, want: "None"},
		{name: "faulted Reset", input: faulted, want: "Reset"},
		{name: "older driver missing field", input: "    Some Other Field : x\n", want: ""},
		{name: "empty output", input: "", want: ""},
		{name: "reboot action", input: "    GPU Recovery Action : Reboot\n", want: "Reboot"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRecoveryAction(tt.input); got != tt.want {
				t.Errorf("parseRecoveryAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNeedsRecoveryAction(t *testing.T) {
	tests := []struct {
		action string
		want   bool
	}{
		{action: "", want: false},
		{action: "None", want: false},
		{action: "none", want: false},
		{action: "N/A", want: false},
		{action: "  None  ", want: false},
		{action: "Reset", want: true},
		{action: "Reboot", want: true},
		{action: "Drain and Reset", want: true},
		{action: "Drain P2P", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			if got := NeedsRecoveryAction(tt.action); got != tt.want {
				t.Errorf("NeedsRecoveryAction(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}
