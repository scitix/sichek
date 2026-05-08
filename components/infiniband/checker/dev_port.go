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
	"fmt"

	"github.com/scitix/sichek/components/infiniband/collector"
)

// devPortLabel returns the human-readable identifier used in checker
// failure lists.  Multi-plane samples (Port>0) get a "/p<n>" suffix so the
// failing plane is visible; legacy single-port records still render as the
// plain device name.
func devPortLabel(hw collector.IBHardWareInfo) string {
	if hw.Port > 0 {
		return fmt.Sprintf("%s/p%d", hw.IBDev, hw.Port)
	}
	return hw.IBDev
}

// uniqueByDev returns one IBHardWareInfo per IB device, used by checkers
// that operate at the device level (firmware, OFED, kernel modules, PCIe
// link characteristics) so multi-plane HCAs do not get reported four
// times.  The chosen sample is deterministic: the entry with the lowest
// port number wins, matching how operators typically pick the "primary"
// plane.
func uniqueByDev(hws map[string]collector.IBHardWareInfo) map[string]collector.IBHardWareInfo {
	uniq := make(map[string]collector.IBHardWareInfo, len(hws))
	for _, hw := range hws {
		if existing, ok := uniq[hw.IBDev]; ok {
			if existing.Port == 0 || (hw.Port > 0 && hw.Port < existing.Port) {
				uniq[hw.IBDev] = hw
			}
			continue
		}
		uniq[hw.IBDev] = hw
	}
	return uniq
}
