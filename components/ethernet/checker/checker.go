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
	"regexp"
	"strconv"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/config"
)

func NewCheckers(cfg *config.EthernetUserConfig, spec *config.EthernetSpecConfig) ([]common.Checker, error) {
	checkers := []common.Checker{
		&L1Checker{
			spec:        spec,
			prevCRC:     make(map[string]int64),
			prevCarrier: make(map[string]int64),
			prevDrops:   make(map[string]int64),
		},
		&L2Checker{
			spec:             spec,
			prevLinkFailures: make(map[string]int64),
			prevActiveSlave:  make(map[string]string),
		},
		&L3Checker{spec: spec},
		&L4Checker{spec: spec},
		&L5Checker{spec: spec},
	}
	// Filter skipped checkers
	ignoredMap := make(map[string]bool)
	if cfg != nil && cfg.Ethernet != nil {
		for _, v := range cfg.Ethernet.IgnoredCheckers {
			ignoredMap[v] = true
		}
	}
	var activeCheckers []common.Checker
	for _, chk := range checkers {
		if !ignoredMap[chk.Name()] {
			activeCheckers = append(activeCheckers, chk)
		}
	}
	return activeCheckers, nil
}


// extractInt parses an integer using regex from a string pattern
func extractInt(input, pattern string) int64 {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		val, _ := strconv.ParseInt(matches[1], 10, 64)
		return val
	}
	return 0
}
