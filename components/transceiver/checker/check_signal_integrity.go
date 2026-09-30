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
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

// badRecommendationPatterns lists (lower-cased) mlxlink Recommendation substrings
// that indicate a physical-layer fault. Extend as more known-bad recommendations
// are observed (e.g. "cable replacement").
var badRecommendationPatterns = []string{
	"bad signal integrity",
}

// SignalIntegrityChecker flags mlxlink "Troubleshooting Info → Recommendation"
// values that indicate physical-layer signal degradation (e.g. "Bad signal
// integrity"). It is intentionally counter-independent: a collapsed eye masked
// by FEC reports a bad recommendation while Symbol Errors / link-error counters
// are still zero, so this checker never inspects those counters.
type SignalIntegrityChecker struct{}

func (c *SignalIntegrityChecker) Name() string { return config.SignalIntegrityCheckerName }

func (c *SignalIntegrityChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for SignalIntegrityChecker")
	}

	tmpl := config.GetCheckItem(c.Name(), "business")
	result := &common.CheckerResult{
		Name:        tmpl.Name,
		Description: tmpl.Description,
		// ErrorName is set unconditionally (not only on the abnormal path) so the
		// Prometheus exporter, which keys reset-then-set on Item+"_"+ErrorName,
		// clears a previously-set abnormal series once the check recovers. A blank
		// ErrorName on the healthy path leaves the stale series stuck at 1 forever.
		ErrorName: tmpl.ErrorName,
		Status:    consts.StatusNormal,
		Level:     consts.LevelInfo,
		Curr:      "OK",
	}

	var abnormalDevices []string
	for _, module := range info.Modules {
		if !module.Present || module.Recommendation == "" {
			continue
		}
		if !matchesBadRecommendation(module.Recommendation) {
			continue
		}

		result.Status = consts.StatusAbnormal
		// Severity follows the network type: business → Critical, management → Warning.
		itemLevel := config.GetCheckItem(c.Name(), module.NetworkType).Level
		if consts.LevelPriority[itemLevel] > consts.LevelPriority[result.Level] {
			result.Level = itemLevel
		}

		dev := module.IBDev
		if dev == "" {
			dev = module.Interface
		}
		result.Detail += fmt.Sprintf(
			"Interface %s (%s): mlxlink recommendation %q — signal integrity degraded, FEC masking errors.\n",
			module.Interface, dev, module.Recommendation,
		)
		abnormalDevices = append(abnormalDevices, module.Interface)
	}

	if result.Status != consts.StatusNormal {
		result.Curr = "abnormal"
		result.Suggestion = tmpl.Suggestion
		result.Device = strings.Join(abnormalDevices, ",")
	}

	return result, nil
}

// matchesBadRecommendation reports whether rec contains any known-bad pattern,
// case-insensitively.
func matchesBadRecommendation(rec string) bool {
	lower := strings.ToLower(rec)
	for _, p := range badRecommendationPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
