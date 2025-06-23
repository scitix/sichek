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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpfs/collector"
	"github.com/scitix/sichek/components/gpfs/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type XStorHealthChecker struct {
	name string
}

func NewXStorHealthChecker(checkerName string) (common.Checker, error) {
	return &XStorHealthChecker{
		name: checkerName,
	}, nil
}

func (c *XStorHealthChecker) Name() string {
	return c.name
}

func (c *XStorHealthChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	result := config.GPFSCheckItems[c.name]
	xstorHealthInfo, ok := data.(*collector.XStorHealthInfo)
	if !ok {
		result.Status = consts.StatusAbnormal
		result.Detail = "invalid gpfsInfo type"
		return &result, fmt.Errorf("invalid gpfsInfo type")
	}

	item, ok := xstorHealthInfo.HealthItems[c.name]
	if !ok {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("Empty %s check result", c.name)
		return &result, nil
	}
	if item.Status != consts.StatusNormal {
		result.Status = consts.StatusAbnormal
		logrus.WithField("components", "GPFS-XStorHealth").Errorf("XStorHealth check %s failed, spec: %s, curr: %s", c.name, item.Spec, item.Curr)
	} else {
		result.Status = consts.StatusNormal
	}
	result.Device = item.Dev
	result.Curr = item.Curr
	result.Spec = item.Spec
	result.Detail = item.Detail

	return &result, nil
}
