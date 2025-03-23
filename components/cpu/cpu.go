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
package cpu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu/checker"
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	commonCfg "github.com/scitix/sichek/config"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg      *config.CPUConfig
	cfgMutex sync.Mutex

	collector common.Collector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	cpuComponent     *component
	cpuComponentOnce sync.Once
)

func NewComponent(cfgFile string) (comp common.Component, err error) {
	cpuComponentOnce.Do(func() {
		cpuComponent, err = new(cfgFile)
		if err != nil {
			panic(err)
		}
	})
	return cpuComponent, nil
}

func new(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	var cfg *config.CPUConfig
	if len(cfgFile) == 0 {
		cfg, err = config.DefaultConfig(ctx)
		if err != nil {
			logrus.WithField("component", "cpu").Errorf("NewComponent get default config failed: %v", err)
			return nil, err
		}
	} else {
		err = cfg.LoadFromYaml(cfgFile)
		if err != nil {
			logrus.WithField("component", "cpu").Errorf("NewComponent load config yaml %s failed: %v", cfgFile, err)
			return nil, err
		}
	}
	collector, err := collector.NewCpuCollector(ctx, cfg)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("NewComponent create collector failed: %v", err)
		return nil, err
	}

	checkers, err := checker.NewCheckers(ctx, cfg)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("NewComponent create checkers failed: %v", err)
		return nil, err
	}

	comp = &component{
		ctx:         ctx,
		cancel:      cancel,
		collector:   collector,
		checkers:    checkers,
		cfg:         cfg,
		cacheBuffer: make([]*common.Result, cfg.GetCacheSize()),
		cacheInfo:   make([]common.Info, cfg.GetCacheSize()),
		cacheSize:   cfg.GetCacheSize(),
	}
	service := common.NewCommonService(ctx, cfg, comp.HealthCheck)
	comp.service = service

	return
}

func (c *component) Name() string {
	return commonCfg.ComponentNameCPU
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	cctx, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()

	info, err := c.collector.Collect(cctx)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("%v", err)
		return nil, err
	}
	cpu_info, ok := info.(*collector.CPUOutput)
	if !ok {
		logrus.WithField("component", "cpu").Errorf("wrong cpu info type")
		return nil, err
	}

	status := commonCfg.StatusNormal
	checkerResults := make([]*common.CheckerResult, 0)
	for _, chker := range c.checkers {
		var checkResult *common.CheckerResult
		eventChecker, ok := chker.(*checker.EventChecker)
		if ok {
			checkResult, err = eventChecker.Check(ctx, cpu_info.EventResults[eventChecker.Name()])
			if err != nil {
				logrus.WithField("component", "cpu").Errorf("failed to check: %v", err)
				continue
			}
		} else {
			checkResult, err = chker.Check(cctx, cpu_info)
			if err != nil {
				logrus.WithField("component", "cpu").Errorf("failed to check: %v", err)
				continue
			}
		}
		checkerResults = append(checkerResults, checkResult)

		if checkResult.Status != commonCfg.StatusNormal {
			status = commonCfg.StatusAbnormal
			logrus.WithField("component", "cpu").Errorf("check Item:%s, status:%s, level:%s", checkResult.Name, checkResult.Status, checkResult.Level)
		}
	}

	level := commonCfg.LevelInfo
	if status == commonCfg.StatusAbnormal {
		level = commonCfg.LevelWarning
	}

	resResult := &common.Result{
		Item:     commonCfg.ComponentNameCPU,
		Node:     cpu_info.HostInfo.Hostname,
		Status:   status,
		Level:    level,
		Checkers: checkerResults,
		Time:     time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = resResult
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if resResult.Status == commonCfg.StatusAbnormal {
		logrus.WithField("component", "cpu").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "cpu").Infof("Health Check PASSED")
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
	var info common.Info
	if c.currIndex == 0 {
		info = c.cacheInfo[c.cacheSize-1]
	} else {
		info = c.cacheInfo[c.currIndex-1]
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
	config, ok := cfg.(*config.CPUConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for cpu")
	}
	c.cfg = config
	c.cfgMutex.Unlock()
	return c.service.Update(ctx, cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}
