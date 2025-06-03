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
	"github.com/scitix/sichek/components/gpfs/collector"
	"github.com/scitix/sichek/components/gpfs/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type EventChecker struct {
	name string
	cfg  *config.GPFSEventConfig
}

func NewEventChecker(cfg common.CheckerSpec) (common.Checker, error) {
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
	gpfsInfo, ok := data.(*collector.GPFSInfo)
	if !ok {
		return nil, fmt.Errorf("invalid gpfsInfo type")
	}

	info := gpfsInfo.EventInfo.FilterResults[c.Name()]
	raw, err := json.Marshal(info)
	if err != nil {
		logrus.Errorf("EventChecker input data cannot JSON marshal: %v", err)
		return nil, err
	}

	totalNum := len(info)
	status := consts.StatusNormal
	if totalNum > 0 {
		status = consts.StatusAbnormal
	}
	result := config.GPFSCheckItems[c.name]
	result.Curr = strconv.Itoa(totalNum)
	result.Status = status
	result.Detail = string(raw)

	return &result, nil
}
