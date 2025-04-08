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
package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/memory/checker"
	"github.com/scitix/sichek/components/memory/collector"
	"github.com/scitix/sichek/components/memory/config"
	"github.com/scitix/sichek/components/memory/metrics"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg      *config.MemoryUserConfig
	cfgMutex sync.Mutex

	collector common.Collector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
	metrics *metrics.MemoryMetrics
}

var (
	memComponent     *component
	memComponentOnce sync.Once
)

func NewComponent(cfgFile string) (comp common.Component, err error) {
	memComponentOnce.Do(func() {
		memComponent, err = newMemoryComponent(cfgFile)
		if err != nil {
			panic(err)
		}
	})
	return memComponent, nil
}

func newMemoryComponent(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	memoryCfg := &config.MemoryUserConfig{}
	err = memoryCfg.LoadUserConfigFromYaml(cfgFile)
	if err != nil {
		logrus.WithField("component", "memory").Errorf("NewMemoryComponent create collector failed: %v", err)
		return nil, err
	}

	collectorPointer, err := collector.NewCollector(ctx, memoryCfg)
	if err != nil {
		logrus.WithField("component", "memory").Errorf("NewMemoryComponent create collector failed: %v", err)
		return nil, err
	}

	checkers := make([]common.Checker, 0)
	for name, spec := range memoryCfg.GetCheckerSpec() {
		memChecker, err := checker.NewMemoryChecker(ctx, spec)
		if err != nil {
			logrus.WithField("component", "memory").Errorf("NewMemoryComponent create checker %s failed: %v", name, err)
			return nil, err
		}
		checkers = append(checkers, memChecker)
	}

	component := &component{
		ctx:         ctx,
		cancel:      cancel,
		collector:   collectorPointer,
		checkers:    checkers,
		cfg:         memoryCfg,
		cacheBuffer: make([]*common.Result, memoryCfg.Memory.CacheSize),
		cacheInfo:   make([]common.Info, memoryCfg.Memory.CacheSize),
		cacheSize:   memoryCfg.Memory.CacheSize,
		metrics:     metrics.NewMemoryMetrics(),
	}
	service := common.NewCommonService(ctx, memoryCfg, component.HealthCheck)
	component.service = service

	return component, nil
}

func (c *component) Name() string {
	return consts.ComponentNameMemory
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	cctx, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()
	info, err := c.collector.Collect(cctx)
	if err != nil {
		logrus.WithField("component", "memory").Errorf("failed to collect memory info: %v", err)
		return nil, err
	}
	memInfo, ok := info.(*collector.Output)
	if !ok {
		logrus.WithField("component", "memory").Errorf("wrong memory collector info type")
		return nil, err
	}
	c.metrics.ExportMetrics(memInfo.Info)
	status := consts.StatusNormal
	checkerResults := make([]*common.CheckerResult, 0)
	for _, each := range c.checkers {
		checkResult, err := each.Check(cctx, memInfo.EventResults[each.Name()])
		if err != nil {
			logrus.WithField("component", "memory").Errorf("failed to check: %v", err)
			continue
		}
		checkerResults = append(checkerResults, checkResult)

		checkerCfg := c.cfg.Memory.Checkers[each.Name()]
		if checkerCfg.Level == consts.LevelCritical && checkResult.Status == consts.StatusAbnormal {
			status = consts.StatusAbnormal
		}
	}

	resResult := &common.Result{
		Item:     consts.ComponentNameMemory,
		Status:   status,
		Level:    consts.LevelCritical,
		Checkers: checkerResults,
		Time:     time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = resResult
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if resResult.Status == consts.StatusAbnormal {
		logrus.WithField("component", "memory").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "memory").Infof("Health Check PASSED")
	}

	return resResult, nil
}

func (c *component) CacheResults(ctx context.Context) ([]*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheBuffer, nil
}

func (c *component) LastResult(ctx context.Context) (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) CacheInfos(ctx context.Context) ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfo, nil
}

func (c *component) LastInfo(ctx context.Context) (common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	info := c.cacheInfo[c.currIndex]
	if c.currIndex == 0 {
		info = c.cacheInfo[c.cacheSize-1]
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

func (c *component) Update(ctx context.Context, cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.MemoryUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for memory")
	}
	c.cfg = configPointer
	c.cfgMutex.Unlock()
	return c.service.Update(ctx, cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}
