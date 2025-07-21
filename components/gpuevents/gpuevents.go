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
package gpuevents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuevents/checker"
	"github.com/scitix/sichek/components/gpuevents/collector"
	"github.com/scitix/sichek/components/gpuevents/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg           *config.GpuCostomEventsUserConfig
	cfgMutex      sync.Mutex
	componentName string
	collector     common.Collector
	checkers      []common.Checker

	cacheMtx          sync.RWMutex
	cacheInfoBuffer   []common.Info
	cacheResultBuffer []*common.Result
	currIndex         int64
	cacheSize         int64

	service *common.CommonService
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
				err = fmt.Errorf("panic occurred when create component gpuevents: %v", r)
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

	userCfg := &config.GpuCostomEventsUserConfig{}
	err = userCfg.LoadUserConfigFromYaml(cfgFile)
	if err != nil {
		logrus.WithField("component", "gpuevents").WithError(err).Error("failed to load GpuCostomEventsUserConfig")
		return nil, err
	}
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "gpuevents").Errorf("NewComponent load spec config failed: %v", err)
		return nil, err
	}
	GpuIndicatorSnapshot, err := collector.NewGpuIndicatorSnapshot(userCfg)
	if err != nil {
		logrus.WithField("component", "gpuevents").WithError(err).Error("failed to create GpuIndicatorSnapshot")
		return nil, err
	}

	hangChecker, err := checker.NewCheckers(userCfg, eventRules)
	if err != nil {
		logrus.WithField("component", "gpuevents").WithError(err).Error("failed to create HangChecker")
		return nil, err
	}

	freqController := common.GetFreqController()
	freqController.RegisterModule(consts.ComponentNameGpuEvents, userCfg)

	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameGpuEvents,
		cfg:           userCfg,

		collector: GpuIndicatorSnapshot,
		checkers:  hangChecker,

		cacheResultBuffer: make([]*common.Result, userCfg.UserConfig.CacheSize),
		cacheInfoBuffer:   make([]common.Info, userCfg.UserConfig.CacheSize),
		currIndex:         0,
		cacheSize:         userCfg.UserConfig.CacheSize,
	}
	component.service = common.NewCommonService(ctx, userCfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil || info == nil {
		logrus.WithField("component", "gpuevents").Error("failed to Collect")
		return &common.Result{}, err
	}
	result := common.Check(ctx, c.componentName, info, c.checkers)
	c.cacheMtx.Lock()
	c.cacheResultBuffer[c.currIndex%c.cacheSize] = result
	c.cacheInfoBuffer[c.currIndex%c.cacheSize] = info
	c.currIndex++
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "gpuevents").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "gpuevents").Infof("Health Check PASSED")
	}

	return result, nil
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
	userCfg, ok := cfg.(*config.GpuCostomEventsUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for gpuevents")
	}
	c.cfg = userCfg
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
	utils.PrintTitle("gpuevents", "-")
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
