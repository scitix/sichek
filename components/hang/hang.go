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
package hang

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/hang/checker"
	"github.com/scitix/sichek/components/hang/collector"
	"github.com/scitix/sichek/components/hang/config"
	"github.com/scitix/sichek/components/hang/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg           *config.HangUserConfig
	cfgMutex      sync.Mutex
	componentName string
	collector     common.Collector
	checker       common.Checker

	cacheMtx          sync.RWMutex
	cacheInfoBuffer   []common.Info
	cacheResultBuffer []*common.Result
	currIndex         int64
	cacheSize         int64

	service *common.CommonService
	metrics *metrics.HangMetrics
}

var (
	hangComponent     common.Component
	hangComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	hangComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component hang: %v", r)
			}
		}()
		hangComponent, err = newComponent(cfgFile, specFile)
	})
	return hangComponent, err
}

func newComponent(cfgFile string, specFile string) (comp common.Component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	hangCfg := &config.HangUserConfig{}
	err = hangCfg.LoadUserConfigFromYaml(cfgFile)
	if err != nil {
		logrus.WithField("component", "hang").WithError(err).Error("failed to load HangUserConfig")
		return nil, err
	}
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "hang").Errorf("NewComponent load spec config failed: %v", err)
		return nil, err
	}
	hangCollector, err := collector.NewHangCollector(hangCfg, eventRules)
	if err != nil {
		logrus.WithField("component", "hang").WithError(err).Error("failed to create HangCollector")
		return nil, err
	}

	hangChecker := checker.NewHangChecker(hangCfg, eventRules)

	freqController := common.GetFreqController()
	freqController.RegisterModule(consts.ComponentNameHang, hangCfg)

	var hangMetrics *metrics.HangMetrics
	if hangCfg.Hang.EnableMetrics {
		hangMetrics = metrics.NewHangMetrics()
	}
	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameHang,
		cfg:           hangCfg,

		collector: hangCollector,
		checker:   hangChecker,

		cacheResultBuffer: make([]*common.Result, hangCfg.Hang.CacheSize),
		cacheInfoBuffer:   make([]common.Info, hangCfg.Hang.CacheSize),
		currIndex:         0,
		cacheSize:         hangCfg.Hang.CacheSize,
		metrics:           hangMetrics,
	}
	component.service = common.NewCommonService(ctx, hangCfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil || info == nil {
		logrus.WithField("component", "hang").Error("failed to Collect")
		return &common.Result{}, err
	}
	hangInfo, ok := info.(*collector.DeviceIndicatorStates)
	if !ok {
		return nil, fmt.Errorf("wrong input of HangChecker")
	}
	if c.cfg.Hang.EnableMetrics {
		c.metrics.ExportMetrics(hangInfo)
	}
	checkRes, err := c.checker.Check(c.ctx, info)
	if err != nil {
		logrus.WithField("component", "hang").WithError(err).Error("failed to Check")
		return &common.Result{}, err
	}
	resResult := &common.Result{
		Item:       consts.ComponentNameHang,
		Node:       "hang",
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
		logrus.WithField("component", "Hang").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "Hang").Infof("Health Check PASSED")
	}

	return resResult, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheResultBuffer, nil
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

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfoBuffer, nil
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

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	hangCfg, ok := cfg.(*config.HangUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for hang")
	}
	c.cfg = hangCfg
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
	checkAllPassed := true
	checkerResults := result.Checkers
	utils.PrintTitle("Hang Error", "-")
	for _, result := range checkerResults {
		if result.Status == consts.StatusAbnormal {
			checkAllPassed = false
			fmt.Printf("\t%s%s%s\n", consts.Red, result.Detail, consts.Reset)
		}
	}
	if checkAllPassed {
		fmt.Printf("%sNo Hang event detected%s\n", consts.Green, consts.Reset)
	}
	return checkAllPassed
}
