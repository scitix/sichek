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
	"github.com/scitix/sichek/components/memory/config"
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
	cfg  *config.MemoryUserConfig

	filter *filter.FileFilter
}

func NewCollector(cfg common.ComponentUserConfig) (*MemoryCollector, error) {
	configPointer, ok := cfg.(*config.MemoryUserConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for Memory")
	}
	filterNames := make([]string, 0)
	regexps := make([]string, 0)
	filesMap := make(map[string]bool)
	files := make([]string, 0)
	for _, checkerCfg := range configPointer.Memory.Checkers {
		_, err := os.Stat(checkerCfg.LogFile)
		if err != nil {
			logrus.WithField("collector", "Memory").Errorf("log file %s not exist for Memory collector", checkerCfg.LogFile)
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

	return &MemoryCollector{
		name:   "MemoryCollector",
		cfg:    configPointer,
		filter: filterPointer,
	}, nil
}

func (c *MemoryCollector) Name() string {
	return c.name
}

func (c *MemoryCollector) GetCfg() common.ComponentUserConfig {
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
