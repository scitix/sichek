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

	cfg           *config.MemoryUserConfig
	cfgMutex      sync.Mutex
	componentName string
	collector     common.Collector
	checkers      []common.Checker

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

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	memComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component memory: %v", r)
			}
		}()
		memComponent, err = newMemoryComponent(cfgFile, specFile)
	})
	return memComponent, err
}

func newMemoryComponent(cfgFile string, specFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	memoryCfg := &config.MemoryUserConfig{}
	err = common.LoadUserConfig(cfgFile, memoryCfg)
	if err != nil || memoryCfg.Memory == nil {
		logrus.WithField("component", "memory").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		return nil, fmt.Errorf("NewMemoryComponent get user config failed")
	}
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "memory").Errorf("failed to NewComponent: %v", err)
		return nil, err
	}
	collectorPointer, err := collector.NewCollector(eventRules)
	if err != nil {
		logrus.WithField("component", "memory").Errorf("NewMemoryComponent create collector failed: %v", err)
		return nil, err
	}

	checkers := make([]common.Checker, 0)
	for name, spec := range eventRules.EventCheckers {
		memChecker, err := checker.NewMemoryChecker(spec)
		if err != nil {
			logrus.WithField("component", "memory").Errorf("NewMemoryComponent create checker %s failed: %v", name, err)
			return nil, err
		}
		checkers = append(checkers, memChecker)
	}
	var memoryMetrics *metrics.MemoryMetrics
	if memoryCfg.Memory.EnableMetrics {
		memoryMetrics = metrics.NewMemoryMetrics()
	}
	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameMemory,
		collector:     collectorPointer,
		checkers:      checkers,
		cfg:           memoryCfg,
		cacheBuffer:   make([]*common.Result, memoryCfg.Memory.CacheSize),
		cacheInfo:     make([]common.Info, memoryCfg.Memory.CacheSize),
		cacheSize:     memoryCfg.Memory.CacheSize,
		metrics:       memoryMetrics,
	}
	service := common.NewCommonService(ctx, memoryCfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	component.service = service

	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "memory").Errorf("failed to collect memory info: %v", err)
		return nil, err
	}
	memInfo, ok := info.(*collector.Output)
	if !ok {
		logrus.WithField("component", "memory").Errorf("wrong memory collector info type")
		return nil, err
	}
	if c.cfg.Memory.EnableMetrics {
		c.metrics.ExportMetrics(memInfo.Info)
	}
	result := common.Check(ctx, c.componentName, memInfo, c.checkers)
	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "memory").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "memory").Infof("Health Check PASSED")
	}

	return result, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheBuffer, nil
}

func (c *component) LastResult() (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	result := c.cacheBuffer[c.currIndex]
	if c.currIndex == 0 {
		result = c.cacheBuffer[c.cacheSize-1]
	}
	return result, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfo, nil
}

func (c *component) LastInfo() (common.Info, error) {
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

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.MemoryUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for memory")
	}
	c.cfg = configPointer
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
	return true
}
