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
)

func TestCheckOFEDVersion(t *testing.T) {
	tests := []struct {
		spec    string
		version string
		expect  bool
	}{
		{">=MLNX_OFED_LINUX-5.9-0.5.6.0", "MLNX_OFED_LINUX-23.10-1.1.9.0", true},    // Version is higher than spec
		{"==MLNX_OFED_LINUX-5.9-0.5.6.0", "MLNX_OFED_LINUX-5.9-0.5.6.0", true},      // Version is equal to spec
		{"MLNX_OFED_LINUX-23.10-1.1.9.0", "MLNX_OFED_LINUX-23.10-1.1.9.0", true},    // Version is equal to spec
		{">=MLNX_OFED_LINUX-5.9-0.5.6.0", "MLNX_OFED_LINUX-23.10.1-1.1.9.0", false}, // invalid current version
		{">=MLNX_OFED_LINUX-5.9-0.5.6.0", "MLNX_OFED_LINUX-23.10-1.1.9.0.9", false}, // invalid current version
		{">=MLNX_OFED_LINUX-5.9-0.5.6.0", "MLNX_OFED_LINUX-23.10.1-1.1.9.0", false}, // invalid spec version
		{"MLNX_OFED_LINUX-23.10-1.*.*.*", "MLNX_OFED_LINUX-23.10-1.1.9.0", true},    // Wildcard matches
		{"MLNX_OFED_LINUX-23.10-*.*.*.*", "MLNX_OFED_LINUX-23.10-1.1.9.0", true},    // Wildcard matches
		{">MLNX_OFED_LINUX-23.10-*.*.*.*", "MLNX_OFED_LINUX-23.9-1.1.9.0", false},   // Wildcard matches
	}

	for _, test := range tests {
		result, _ := checkOFEDVersion(test.spec, test.version)
		if result != test.expect {
			t.Errorf("compareVersion(%s, %s) = %v (expected %v)\n", test.spec, test.version, result, test.expect)

		}
	}
}
