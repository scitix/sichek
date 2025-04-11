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
	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/components/memory/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

type MemoryChecker struct {
	name string
	cfg  *config.MemoryEventConfig
}

func NewMemoryChecker(cfg common.CheckerSpec) (common.Checker, error) {
	gpfsCfg, ok := cfg.(*config.MemoryEventConfig)
	if !ok {
		return nil, fmt.Errorf("invalid MemoryChecker config type")
	}

	return &MemoryChecker{
		name: gpfsCfg.Name,
		cfg:  gpfsCfg,
	}, nil
}

func (c *MemoryChecker) Name() string {
	return c.name
}

func (c *MemoryChecker) GetSpec() common.CheckerSpec {
	return c.cfg
}

func (c *MemoryChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	memoryInfo, ok := data.(*collector.Output)
	if !ok {
		return nil, fmt.Errorf("invalid memoryInfo type")
	}

	info := memoryInfo.EventResults[c.Name()]
	raw, err := json.Marshal(info)
	if err != nil {
		logrus.Errorf("MemoryChecker input data cannot JSON marshal: %v", err)
		return nil, err
	}

	totalNum := len(info)
	status := consts.StatusNormal
	if totalNum > 0 {
		status = consts.StatusAbnormal
	}

	return &common.CheckerResult{
		Name:        c.cfg.Name,
		Description: c.cfg.Description,
		Device:      "memory",
		Spec:        "0",
		Curr:        strconv.Itoa(totalNum),
		Status:      status,
		Level:       c.cfg.Level,
		Detail:      string(raw),
		ErrorName:   c.cfg.Name,
	}, nil
}
