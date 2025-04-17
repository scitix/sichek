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
	"github.com/scitix/sichek/components/dmesg/config"
	"github.com/scitix/sichek/pkg/utils/filter"

	"github.com/sirupsen/logrus"
)

type DmesgCollector struct {
	name string
	cfg  common.ComponentUserConfig

	filter *filter.Filter
}

func NewDmesgCollector(cfg *config.DmesgUserConfig) (*DmesgCollector, error) {

	if len(cfg.Dmesg.EventCheckers) == 0 {
		return nil, fmt.Errorf("no Dmesg Collector indicate in yaml config")
	}
	regexpName := make([]string, 0, len(cfg.Dmesg.EventCheckers))
	regexp := make([]string, 0, len(cfg.Dmesg.EventCheckers))

	for _, checkersCfg := range cfg.Dmesg.EventCheckers {
		regexpName = append(regexpName, checkersCfg.Name)
		regexp = append(regexp, checkersCfg.Regexp)
	}

	filterPointer, err := filter.NewFilter(
		regexpName,
		regexp,
		cfg.Dmesg.DmesgFileName,
		cfg.Dmesg.DmesgCmd,
		5000,
	)
	if err != nil {
		logrus.WithError(err).Error("failed to create filter in DmesgCollector")
		return nil, err
	}

	return &DmesgCollector{
		name:   "DmesgCollector",
		cfg:    cfg,
		filter: filterPointer,
	}, nil
}

func (c *DmesgCollector) Name() string {
	return c.name
}

func (c *DmesgCollector) GetCfg() common.ComponentUserConfig {
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
