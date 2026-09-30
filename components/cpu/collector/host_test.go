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
	"testing"
)

func TestHostInfo_Get(t *testing.T) {
	hostInfo := &HostInfo{}
	err := hostInfo.Get()
	if err != nil {
		t.Errorf("Failed to get host information: %v", err)
	}
	t.Logf("HostInfo: %v", hostInfo.ToString())

	uptime, err := GetUptime()
	if err != nil {
		t.Errorf("Failed to get uptime: %v", err)
	}
	t.Logf("Uptime: %v", uptime)
}
