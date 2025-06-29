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
package dmesg

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/dmesg/checker"
	"github.com/scitix/sichek/components/dmesg/collector"
	"github.com/scitix/sichek/components/dmesg/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string

	cfg      *config.DmesgUserConfig
	cfgMutex sync.Mutex

	collector common.Collector
	checker   common.Checker

	cacheMtx          sync.RWMutex
	cacheInfoBuffer   []common.Info
	cacheResultBuffer []*common.Result
	currIndex         int64
	cacheSize         int64

	service *common.CommonService
}

var (
	dmesgComponent     common.Component
	dmesgComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	dmesgComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component dmesg: %v", r)
			}
		}()
		dmesgComponent, err = newComponent(cfgFile, specFile)
	})
	return dmesgComponent, err
}

func newComponent(cfgFile string, specFile string) (comp common.Component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	dmsgCfg := &config.DmesgUserConfig{}
	err = common.LoadUserConfig(cfgFile, dmsgCfg)
	if err != nil || dmsgCfg.Dmesg == nil {
		logrus.WithField("component", "dmesg").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		return nil, fmt.Errorf("NewDmesgComponent get user config failed")
	}
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "dmesg").Errorf("failed to NewComponent: %v", err)
		return nil, err
	}
	collectorPointer, err := collector.NewDmesgCollector(eventRules)
	if err != nil {
		logrus.WithField("component", "dmesg").WithError(err).Error("failed to create DmesgCollector")
	}

	dmesgChecker := checker.NewDmesgChecker(eventRules)

	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameDmesg,
		cfg:           dmsgCfg,

		collector: collectorPointer,
		checker:   dmesgChecker,

		cacheResultBuffer: make([]*common.Result, dmsgCfg.Dmesg.CacheSize),
		cacheInfoBuffer:   make([]common.Info, dmsgCfg.Dmesg.CacheSize),
		currIndex:         0,
		cacheSize:         dmsgCfg.Dmesg.CacheSize,
	}
	component.service = common.NewCommonService(ctx, dmsgCfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "dmesg").WithError(err).Error("failed to Collect(ctx)")
		return &common.Result{}, err
	}

	checkRes, err := c.checker.Check(ctx, info)
	if err != nil {
		logrus.WithField("component", "dmesg").WithError(err).Error("failed to Check()")
		return &common.Result{}, err
	}

	resResult := &common.Result{
		Item:       consts.ComponentNameDmesg,
		Status:     checkRes.Status,
		Level:      checkRes.Level,
		Suggestion: checkRes.Suggestion,
		Checkers:   []*common.CheckerResult{checkRes},
		Time:       time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheResultBuffer[c.currIndex%c.cacheSize] = resResult
	c.cacheInfoBuffer[c.currIndex%c.cacheSize] = info
	c.currIndex++
	c.cacheMtx.Unlock()
	if resResult.Status == consts.StatusAbnormal {
		logrus.WithField("component", "dmesg").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "dmesg").Infof("Health Check PASSED")
	}

	return resResult, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheResultBuffer, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheInfoBuffer, nil
}

func (c *component) LastResult() (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheResultBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheResultBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) LastInfo() (common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	info := c.cacheInfoBuffer[c.currIndex]
	if c.currIndex == 0 {
		info = c.cacheInfoBuffer[c.cacheSize-1]
	}
	return info, nil
}

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	dmsgCfg, ok := cfg.(*config.DmesgUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for dmesg")
	}
	c.cfg = dmsgCfg
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}

func (c *component) GetTimeout() time.Duration {
	return c.cfg.GetQueryInterval().Duration
}
func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	dmesgEvent := make(map[string]string)
	checkAllPassed := true
	checkerResults := result.Checkers
	for _, result := range checkerResults {
		switch result.Name {
		case "DmesgErrorChecker":
			if result.Status == consts.StatusAbnormal {
				checkAllPassed = false
				dmesgEvent["DmesgErrorChecker"] = fmt.Sprintf("%s%s%s", consts.Red, result.Detail, consts.Reset)
			}
		}
	}

	utils.PrintTitle("Dmesg", "-")
	if len(dmesgEvent) == 0 {
		fmt.Printf("%sNo Dmesg event detected%s\n", consts.Green, consts.Reset)
		return checkAllPassed
	}
	for n := range dmesgEvent {
		fmt.Printf("\tDetected %d Kernel Events:\n %s\n", len(dmesgEvent), dmesgEvent[n])
	}
	return checkAllPassed
}
