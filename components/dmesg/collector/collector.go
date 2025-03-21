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
	"fmt"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/dmesg/checker"
	DmesgCfg "github.com/scitix/sichek/components/dmesg/config"
	"github.com/scitix/sichek/pkg/utils/filter"

	"github.com/sirupsen/logrus"
)

type DmesgCollector struct {
	name string
	cfg  common.ComponentConfig

	filter *filter.Filter
}

func NewDmesgCollector(ctx context.Context, cfg common.ComponentConfig) (*DmesgCollector, error) {
	config, ok := cfg.(*DmesgCfg.DmesgConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for GPFS")
	}

	if len(config.Dmesg.CheckerConfigs) == 0 {
		return nil, fmt.Errorf("No Dmesg Collector indicate in yaml config")
	}
	regexpName := make([]string, 0, len(config.Dmesg.CheckerConfigs))
	regexp := make([]string, 0, len(config.Dmesg.CheckerConfigs))

	for _, checkers_cfg := range config.Dmesg.CheckerConfigs {
		regexpName = append(regexpName, checkers_cfg.Name)
		regexp = append(regexp, checkers_cfg.Regexp)
	}

	filter, err := filter.NewFilter(
		regexpName,
		regexp,
		config.Dmesg.DmesgFileName,
		config.Dmesg.DmesgCmd,
		5000,
	)
	if err != nil {
		logrus.WithError(err).Error("failed to create filter in DmesgCollector")
		return nil, err
	}

	return &DmesgCollector{
		name:   "DmesgCollector",
		cfg:    cfg,
		filter: filter,
	}, nil
}

func (c *DmesgCollector) Name() string {
	return c.name
}

func (c *DmesgCollector) GetCfg() common.ComponentConfig {
	return c.cfg
}

func (c *DmesgCollector) Collect(ctx context.Context) (common.Info, error) {
	filterRes := c.filter.Check()

	var res checker.DmesgInfo
	res.Time = time.Now()
	for i := 0; i < len(filterRes); i++ {
		res.Name = append(res.Name, filterRes[i].Name)
		res.Regexp = append(res.Regexp, filterRes[i].Regex)
		res.FileName = append(res.FileName, filterRes[i].FileName)
		res.Raw = append(res.Raw, filterRes[i].Line)
	}
	return &res, nil
}
