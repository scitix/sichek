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
	"github.com/scitix/sichek/components/gpfs/config"
	commonCfg "github.com/scitix/sichek/config"
	"github.com/scitix/sichek/pkg/utils/filter"

	"github.com/sirupsen/logrus"
)

func NewCheckers(ctx context.Context, cfg common.ComponentConfig) ([]common.Checker, error) {
	gpfs_cfg, ok := cfg.(*config.GpfsConfig)
	if !ok {
		err := fmt.Errorf("invalid config type for gpfs checker")
		logrus.WithField("component", "gpfs").Error(err)
		return nil, err
	}

	checkers := make([]common.Checker, 0)
	for name, config := range gpfs_cfg.EventCheckers {
		checker, err := NewEventChecker(ctx, config)
		if err != nil {
			logrus.WithField("component", "gpfs").Errorf("create event checker %s failed: %v", name, err)
			return nil, err
		}
		checkers = append(checkers, checker)
	}

	return checkers, nil
}

type EventChecker struct {
	name string
	cfg  *config.GPFSEventConfig
}

func NewEventChecker(ctx context.Context, cfg common.CheckerSpec) (common.Checker, error) {
	gpfsCfg, ok := cfg.(*config.GPFSEventConfig)
	if !ok {
		return nil, fmt.Errorf("invalid EventChecker config type")
	}

	return &EventChecker{
		name: gpfsCfg.Name,
		cfg:  gpfsCfg,
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
		return nil, fmt.Errorf("wrong input of EventChecker")
	}
	raw, err := json.Marshal(info)
	if err != nil {
		logrus.Errorf("EventChecker input data cannot JSON marshal: %v", err)
		return nil, err
	}

	total_num := len(info)
	status := commonCfg.StatusNormal
	if total_num > 0 {
		status = commonCfg.StatusAbnormal
	}
	result := config.GPFSCheckItems[c.name]
	result.Curr = strconv.Itoa(total_num)
	result.Status = status
	result.Detail = string(raw)

	return &result, nil
}
