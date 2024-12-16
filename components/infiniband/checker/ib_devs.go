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
	commonCfg "github.com/scitix/sichek/config"
)

type IBDevsChecker struct {
	name        string
	spec        *config.InfinibandHCASpec
	description string
}

func NewIBDevsChecker(specCfg *config.InfinibandHCASpec) (common.Checker, error) {
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
	failed_hcas := make([]string, 0)
	IBDevSet := make(map[string]struct{})
	for _, hca := range c.spec.IBDevs {
		IBDevSet[hca] = struct{}{}
	}

	for _, hca := range infinibandInfo.IBDevs {
		if _, found := IBDevSet[hca]; !found {
			failed_hcas = append(failed_hcas, hca)
		}
	}

	if len(failed_hcas) > 0 {
		result.Status = commonCfg.StatusAbnormal
		result.Device = strings.Join(failed_hcas, ",")
		result.Detail = fmt.Sprintf("Unexpected IB devices %v, expected IB devices : %v", infinibandInfo.IBDevs, c.spec.IBDevs)
	} else {
		result.Status = commonCfg.StatusNormal
	}

	return &result, nil
}
