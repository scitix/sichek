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
	"github.com/scitix/sichek/config"
	"github.com/scitix/sichek/config/dmesg"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg      *dmesg.DmesgConfig
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

func NewComponent(componentConfig *config.ComponentConfig) (comp common.Component, err error) {
	dmesgComponentOnce.Do(func() {
		dmesgComponent, err = newComponent(componentConfig)
		if err != nil {
			panic(err)
		}
	})
	return dmesgComponent, nil
}

func newComponent(componentConfig *config.ComponentConfig) (comp common.Component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	cfg, _ := componentConfig.GetConfigByComponentName(consts.ComponentNameDmesg)
	if cfg == nil {
		logrus.WithField("component", "dmesg").Errorf("NewComponent get config failed: %v", err)
		return nil, err
	}
	dmsgCfg, ok := cfg.(*dmesg.DmesgConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for CPU component")
	}

	collector, err := collector.NewDmesgCollector(ctx, dmsgCfg)
	if err != nil {
		logrus.WithField("component", "dmesg").WithError(err).Error("failed to create DmesgCollector")
	}

	checker := checker.NewDmesgChecker(dmsgCfg)

	component := &component{
		ctx:    ctx,
		cancel: cancel,

		cfg: dmsgCfg,

		collector: collector,
		checker:   checker,

		cacheResultBuffer: make([]*common.Result, dmsgCfg.CacheSize),
		cacheInfoBuffer:   make([]common.Info, dmsgCfg.CacheSize),
		currIndex:         0,
		cacheSize:         dmsgCfg.CacheSize,
	}
	component.service = common.NewCommonService(ctx, dmsgCfg, component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return consts.ComponentNameDmesg
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	_, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "dmesg").WithError(err).Error("failed to Collect()")
		return &common.Result{}, err
	}

	checkRes, err := c.checker.Check(c.ctx, info)
	if err != nil {
		logrus.WithField("component", "dmesg").WithError(err).Error("failed to Check()")
		return &common.Result{}, err
	}

	resResult := &common.Result{
		Item:       consts.ComponentNameDmesg,
		Node:       "dmesg",
		Status:     checkRes.Status,
		Level:      consts.LevelCritical,
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

func (c *component) CacheResults(ctx context.Context) ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheResultBuffer, nil
}

func (c *component) CacheInfos(ctx context.Context) ([]common.Info, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheInfoBuffer, nil
}

func (c *component) LastResult(ctx context.Context) (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheResultBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheResultBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) LastInfo(ctx context.Context) (common.Info, error) {
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

func (c *component) Start(ctx context.Context) <-chan *common.Result {
	return c.service.Start(ctx)
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(ctx context.Context, cfg common.ComponentConfig) error {
	c.cfgMutex.Lock()
	dmsgCfg, ok := cfg.(*dmesg.DmesgConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for dmesg")
	}
	c.cfg = dmsgCfg
	c.cfgMutex.Unlock()
	return c.service.Update(ctx, cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}
