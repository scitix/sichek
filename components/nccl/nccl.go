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
package nccl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nccl/checker"
	"github.com/scitix/sichek/components/nccl/collector"
	"github.com/scitix/sichek/components/nccl/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg      *config.NCCLUserConfig
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
	ncclComponent     common.Component
	ncclComponentOnce sync.Once
)

func NewComponent(cfgFile string) (comp common.Component, err error) {
	ncclComponentOnce.Do(func() {
		ncclComponent, err = newComponent(cfgFile)
		if err != nil {
			panic(err)
		}
	})
	return ncclComponent, nil
}

func newComponent(cfgFile string) (comp common.Component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	ncclCfg := &config.NCCLUserConfig{}
	err = ncclCfg.LoadUserConfigFromYaml(cfgFile)
	if err != nil {
		logrus.WithField("component", "nccl").WithError(err).Error("failed to load NCCL config")
		return nil, err
	}
	ncclCollector, err := collector.NewNCCLCollector(ncclCfg)
	if err != nil {
		logrus.WithField("component", "nccl").WithError(err).Error("failed to create NCCLCollector")
		return nil, err
	}

	ncclChecker := checker.NewNCCLChecker(ncclCfg)

	component := &component{
		ctx:    ctx,
		cancel: cancel,

		cfg: ncclCfg,

		collector: ncclCollector,
		checker:   ncclChecker,

		cacheResultBuffer: make([]*common.Result, ncclCfg.NCCL.CacheSize),
		cacheInfoBuffer:   make([]common.Info, ncclCfg.NCCL.CacheSize),
		currIndex:         0,
		cacheSize:         ncclCfg.NCCL.CacheSize,
	}
	component.service = common.NewCommonService(ctx, ncclCfg, component.Name(), component.GetTimeout(), component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return consts.ComponentNameNCCL
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "nccl").WithError(err).Error("failed to Collect(ctx)")
		return &common.Result{}, err
	}

	checkRes, err := c.checker.Check(ctx, info)
	if err != nil {
		logrus.WithField("component", "nccl").WithError(err).Error("failed to Check()")
		return &common.Result{}, err
	}

	resResult := &common.Result{
		Item:       consts.ComponentNameNCCL,
		Node:       "NCCLLog",
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
		logrus.WithField("component", "nccl").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "nccl").Infof("Health Check PASSED")
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
	var info common.Info
	if c.currIndex == 0 {
		info = c.cacheInfoBuffer[c.cacheSize-1]
	} else {
		info = c.cacheInfoBuffer[c.currIndex-1]
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
	config, ok := cfg.(*config.NCCLUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for nccl")
	}
	c.cfg = config
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}

func (c *component) GetTimeout() time.Duration {
	return c.cfg.GetQueryInterval() * time.Second
}
