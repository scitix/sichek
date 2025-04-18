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
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type EventChecker struct {
	name string
	cfg  *config.CPUEventConfig
}

func NewEventChecker(cpuEventCfg *config.CPUEventConfig) (common.Checker, error) {
	return &EventChecker{
		name: cpuEventCfg.Name,
		cfg:  cpuEventCfg,
	}, nil
}

func (c *EventChecker) Name() string {
	return c.name
}

func (c *EventChecker) GetSpec() common.CheckerSpec {
	return c.cfg
}

func (c *EventChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuInfo, ok := data.(*collector.CPUOutput)
	if !ok {
		return nil, fmt.Errorf("invalid cpuInfo type")
	}
	info := cpuInfo.EventResults[c.Name()]
	raw, err := json.Marshal(info)
	if err != nil {
		logrus.Errorf("CpuEventChecker input data cannot JSON marshal: %v", err)
		return nil, err
	}

	totalNum := len(info)
	status := consts.StatusNormal
	suggestion := ""
	if totalNum > 0 {
		status = consts.StatusAbnormal
		suggestion = c.cfg.Suggestion
	}

	return &common.CheckerResult{
		Name:        c.cfg.Name,
		Description: c.cfg.Description,
		Spec:        "0",
		Curr:        strconv.Itoa(totalNum),
		Status:      status,
		Level:       c.cfg.Level,
		Suggestion:  suggestion,
		Detail:      string(raw),
		ErrorName:   c.cfg.Name,
	}, nil
}
