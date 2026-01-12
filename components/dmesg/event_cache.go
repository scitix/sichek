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
package dmesg

import (
	"regexp"
	"strconv"
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

const (
	MaxDetailLines = 3
)

// RuntimeEventRule Config -> Runtime, only compile once at init
type RuntimeEventRule struct {
	Name            string
	RegexObj        *regexp.Regexp
	EventRuleConfig *common.EventRuleConfig
}

// EventCache is used to store the result of event matching in a HealthCheck cycle
// It is reset after Drain() called in each HealthCheck cycle
type EventCache struct {
	mu sync.Mutex

	// immutable
	runtimeEventRules map[string]RuntimeEventRule

	// mutable per-cycle state
	result          *common.Result
	eventsResultMap map[string]*common.CheckerResult
}

func NewEventCache(eventRules common.EventRuleGroup) *EventCache {
	runtimeRules := make(map[string]RuntimeEventRule, len(eventRules))

	for name, eventRule := range eventRules {
		re, err := regexp.Compile(eventRule.Regexp)
		if err != nil {
			logrus.WithField("EventCache", "NewEventCache").WithError(err).Errorf("invalid regexp for rule %s", name)
			continue
		}
		runtimeRules[name] = RuntimeEventRule{
			Name:            name,
			RegexObj:        re,
			EventRuleConfig: eventRule,
		}
	}

	eventCache := &EventCache{
		runtimeEventRules: runtimeRules,
	}

	eventCache.reset()
	return eventCache
}

func (c *EventCache) MatchLine(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for name, eventRule := range c.runtimeEventRules {
		if eventRule.RegexObj.MatchString(line) {
			c.add(name, line)
		}
	}
}

func (c *EventCache) add(name, detail string) {
	if entry, exists := c.eventsResultMap[name]; !exists {
		eventRule := c.runtimeEventRules[name]
		checkResult := &common.CheckerResult{
			Name:        name,
			Description: eventRule.EventRuleConfig.Description,
			Curr:        "1",
			Device:      "",
			Status:      consts.StatusAbnormal,
			Level:       eventRule.EventRuleConfig.Level,
			ErrorName:   name,
			Suggestion:  eventRule.EventRuleConfig.Suggestion,
			Detail:      detail,
		}
		c.eventsResultMap[name] = checkResult
		c.result.Checkers = append(c.result.Checkers, checkResult)

		// Update overall status if it's the first abnormal event
		c.result.Status = consts.StatusAbnormal
		if eventRule.EventRuleConfig.Level > c.result.Level {
			c.result.Level = eventRule.EventRuleConfig.Level
		}
	} else {
		// Already exists, increment count
		curr, _ := strconv.Atoi(entry.Curr)
		curr++
		entry.Curr = strconv.Itoa(curr)

		if curr <= MaxDetailLines {
			entry.Detail += "\n" + detail
		}
	}
}

// Drain returns the current cycle result and resets the cache for the next cycle
func (c *EventCache) Drain() *common.Result {
	c.mu.Lock()
	defer c.mu.Unlock()

	ev := c.result
	c.reset()
	return ev
}

func (c *EventCache) reset() {
	c.result = &common.Result{
		Status:   consts.StatusNormal,
		Level:    consts.LevelInfo,
		Checkers: make([]*common.CheckerResult, 0),
	}
	c.eventsResultMap = make(map[string]*common.CheckerResult)
}
