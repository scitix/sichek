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
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
)

type IBDevsChecker struct {
	name        string
	spec        *config.InfinibandSpec
	description string
}

func NewIBDevsChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &IBDevsChecker{
		name: config.CheckIBDevs,
		spec: specCfg,
	}, nil
}

func (c *IBDevsChecker) Name() string {
	return c.name
}

func (c *IBDevsChecker) Description() string {
	return c.description
}

func (c *IBDevsChecker) GetSpec() common.CheckerSpec {
	return nil
}

func (c *IBDevsChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	result := config.InfinibandCheckItems[c.name]

	var mismatchPairs []string
	for expectedMlx5, expectedIb := range c.spec.IBDevs {
		actualIb, found := infinibandInfo.IBDevs[expectedMlx5]
		if !found {
			mismatchPairs = append(mismatchPairs, fmt.Sprintf("%s (missing)", expectedMlx5))
			continue
		}
		if actualIb != expectedIb {
			mismatchPairs = append(mismatchPairs, fmt.Sprintf("%s -> %s (expected %s)", expectedMlx5, actualIb, expectedIb))
		}
	}

	if len(mismatchPairs) > 0 {
		result.Status = consts.StatusAbnormal
		result.Device = strings.Join(mismatchPairs, ",")
		result.Detail = fmt.Sprintf("Mismatched IB devices %v, expected: %v", infinibandInfo.IBDevs, c.spec.IBDevs)
	} else {
		result.Status = consts.StatusNormal
	}
	var specSlice []string
	for mlxDev, ibDev := range c.spec.IBDevs {
		specSlice = append(specSlice, mlxDev+":"+ibDev)
	}
	result.Spec = strings.Join(specSlice, ",")
	var ibDevsSlice []string
	for mlxDev, ibDev := range infinibandInfo.IBDevs {
		ibDevsSlice = append(ibDevsSlice, mlxDev+":"+ibDev)
	}
	result.Curr = strings.Join(ibDevsSlice, ",")
	return &result, nil
}
