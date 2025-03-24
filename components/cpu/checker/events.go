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
	"github.com/scitix/sichek/config/cpu"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils/filter"

	"github.com/sirupsen/logrus"
)

type EventChecker struct {
	name string
	cfg  *cpu.CPUEventConfig
}

func NewEventChecker(ctx context.Context, cfg common.CheckerSpec) (common.Checker, error) {
	cpuEventCfg, ok := cfg.(*cpu.CPUEventConfig)
	if !ok {
		return nil, fmt.Errorf("invalid CPU event checker config type")
	}

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
	info, ok := data.([]*filter.FilterResult)
	if !ok {
		return nil, fmt.Errorf("wrong input of CPU event checker")
	}
	raw, err := json.Marshal(info)
	if err != nil {
		logrus.Errorf("GPFSChecker input data cannot JSON marshal: %v", err)
		return nil, err
	}

	total_num := len(info)
	status := consts.StatusNormal
	suggestion := ""
	if total_num > 0 {
		status = consts.StatusAbnormal
		suggestion = c.cfg.Suggestion
	}

	return &common.CheckerResult{
		Name:        c.cfg.Name,
		Description: c.cfg.Description,
		Spec:        "0",
		Curr:        strconv.Itoa(total_num),
		Status:      status,
		Level:       c.cfg.Level,
		Suggestion:  suggestion,
		Detail:      string(raw),
		ErrorName:   c.cfg.Name,
	}, nil
}
