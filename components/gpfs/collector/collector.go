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
	"os"
	"time"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpfs/config"
	"github.com/scitix/sichek/pkg/utils/filter"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type GPFSCollector struct {
	name string
	cfg  *config.GpfsEventRule

	filter *filter.FileFilter
	xstorHealth XStorHealthInfo
}

func NewGPFSCollector(cfg *config.GpfsEventRule) (*GPFSCollector, error) {
	filterNames := make([]string, 0)
	regexps := make([]string, 0)
	filesMap := make(map[string]bool)
	files := make([]string, 0)
	for _, checkerCfg := range cfg.EventCheckers {
		_, err := os.Stat(checkerCfg.LogFile)
		if err != nil {
			logrus.WithField("collector", "GPFS").Errorf("log file %s not exist for GPFS collector", checkerCfg.LogFile)
			continue
		}
		filterNames = append(filterNames, checkerCfg.Name)
		if _, exist := filesMap[checkerCfg.LogFile]; !exist {
			files = append(files, checkerCfg.LogFile)
			filesMap[checkerCfg.LogFile] = true
		}
		regexps = append(regexps, checkerCfg.Regexp)
	}

	filterPointer, err := filter.NewFileFilter(filterNames, regexps, files, 1)
	if err != nil {
		return nil, err
	}
	collector := &GPFSCollector{
		name:   "GPFSCollector",
		cfg:    cfg,
		filter: filterPointer,
	}
	
	if len(cfg.XStorHealthCheckers) > 0 {
		collector.xstorHealth = XStorHealthInfo{
			HealthItems: make(map[string]*GPFSXStorHealthItem),
		}
	}

	return collector, nil
}

func (c *GPFSCollector) Name() string {
	return c.name
}

func (c *GPFSCollector) getXStorHealthInfo(ctx context.Context) error {
	if len(c.cfg.XStorHealthCheckers) == 0 {
		return nil
	}

	xstorHealthOutput, err := utils.ExecCommand(ctx, "xstor-health", "basic-check", "--output-format", "sicheck")
	if err != nil {
		return fmt.Errorf("exec xstor-health failed: %v", err)
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
	return nil
}

func (c *GPFSCollector) Collect(ctx context.Context) (common.Info, error) {
	filterRes := c.filter.Check()
	filterResMap := make(map[string][]*filter.FilterResult)
	for _, res := range filterRes {
		filterResMap[res.Name] = append(filterResMap[res.Name], &res)
	}
	event_info := EventInfo{
		FilterResults: filterResMap,
	}
	err := c.getXStorHealthInfo(ctx)
	if err != nil {
		logrus.WithField("component", "GPFS-Collector").Errorf("failed to get XStorHealthInfo: %v", err)
	}

	info := &GPFSInfo{
		Time:          	 time.Now(),
		EventInfo:		 event_info,
		XStorHealthInfo: c.xstorHealth,
	}

	return info, nil
}
