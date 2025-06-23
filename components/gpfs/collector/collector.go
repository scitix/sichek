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
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type GPFSCollector struct {
	name        string
	xstorHealth *XStorHealthInfo
}

func NewGPFSCollector() (*GPFSCollector, error) {
	collector := &GPFSCollector{
		name: "GPFSCollector",
		xstorHealth: &XStorHealthInfo{
			HealthItems: make(map[string]*GPFSXStorHealthItem),
		},
	}
	return collector, nil
}

func (c *GPFSCollector) Name() string {
	return c.name
}

func (c *GPFSCollector) Collect(ctx context.Context) (*XStorHealthInfo, error) {
	xstorHealthOutput, err := utils.ExecCommand(ctx, "xstor-health", "basic-check", "--output-format", "sicheck")
	if err != nil {
		return nil, fmt.Errorf("exec xstor-health failed: %v", err)
	}
	healthItemStrs := strings.Split(string(xstorHealthOutput), "\n")
	for _, itemStr := range healthItemStrs {
		if len(itemStr) == 0 {
			continue
		}
		var item GPFSXStorHealthItem
		err := json.Unmarshal([]byte(itemStr), &item)
		if err != nil {
			logrus.WithField("component", "GPFS-Collector").Errorf("failed to unmarshal %s to GPFSXStorHealthItem", itemStr)
			continue
		}
		c.xstorHealth.HealthItems[item.Item] = &item
	}
	return c.xstorHealth, nil
}
