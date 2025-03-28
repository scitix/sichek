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
	"os"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/config/memory"
	"github.com/scitix/sichek/pkg/utils/filter"

	"github.com/sirupsen/logrus"
)

type Output struct {
	Info         *MemoryInfo                       `json:"info"`
	EventResults map[string][]*filter.FilterResult `json:"event_results"`
	Time         time.Time
}

func (o *Output) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}

type MemoryCollector struct {
	name string
	cfg  *memory.MemoryConfig

	filter *filter.FileFilter
}

func NewCollector(ctx context.Context, cfg common.ComponentConfig) (*MemoryCollector, error) {
	config, ok := cfg.(*memory.MemoryConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for Memory")
	}
	filterNames := make([]string, 0)
	regexps := make([]string, 0)
	files_map := make(map[string]bool)
	files := make([]string, 0)
	for _, checker_cfg := range config.Checkers {
		_, err := os.Stat(checker_cfg.LogFile)
		if err != nil {
			logrus.WithField("collector", "Memory").Errorf("log file %s not exist for Memory collector", checker_cfg.LogFile)
			continue
		}
		filterNames = append(filterNames, checker_cfg.Name)
		if _, exist := files_map[checker_cfg.LogFile]; !exist {
			files = append(files, checker_cfg.LogFile)
			files_map[checker_cfg.LogFile] = true
		}
		regexps = append(regexps, checker_cfg.Regexp)
	}

	filter, err := filter.NewFileFilter(filterNames, regexps, files, 1)
	if err != nil {
		return nil, err
	}

	return &MemoryCollector{
		name:   "MemoryCollector",
		cfg:    config,
		filter: filter,
	}, nil
}

func (c *MemoryCollector) Name() string {
	return c.name
}

func (c *MemoryCollector) GetCfg() common.ComponentConfig {
	return c.cfg
}

func (c *MemoryCollector) Collect(ctx context.Context) (common.Info, error) {
	filterRes := c.filter.Check()
	filterResMap := make(map[string][]*filter.FilterResult)
	for _, res := range filterRes {
		filterResMap[res.Name] = append(filterResMap[res.Name], &res)
	}
	info := &MemoryInfo{}
	err := info.Get()
	if err != nil {
		logrus.WithField("collector", "memory").Errorf("get memory info failed: %v", err)
		return nil, err
	}

	output := &Output{
		Info:         info,
		EventResults: filterResMap,
		Time:         time.Now(),
	}

	return output, nil
}
