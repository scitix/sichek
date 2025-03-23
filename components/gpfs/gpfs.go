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
package gpfs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpfs/checker"
	"github.com/scitix/sichek/components/gpfs/collector"
	"github.com/scitix/sichek/components/gpfs/config"
	commonCfg "github.com/scitix/sichek/config"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg      *config.GpfsConfig
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
	gpfsComponent     *component
	gpfsComponentOnce sync.Once
)

func NewGpfsComponent(cfgFile string) (comp common.Component, err error) {
	gpfsComponentOnce.Do(func() {
		gpfsComponent, err = newGpfsComponent(cfgFile)
		if err != nil {
			panic(err)
		}
	})
	return gpfsComponent, nil
}

func newGpfsComponent(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.GpfsConfig{}
	if len(cfgFile) == 0 {
		err := commonCfg.DefaultConfig(commonCfg.ComponentNameGpfs, cfg)
		if err != nil {
			logrus.WithField("component", "gpfs").Errorf("NewGpfsComponent get default config failed: %v", err)
			return nil, err
		}
	} else {
		err = commonCfg.LoadFromYaml(cfgFile,cfg)
		if err != nil {
			logrus.WithField("component", "gpfs").Errorf("NewGpfsComponent load config yaml %s failed: %v", cfgFile, err)
			return nil, err
		}
	}
	collector, err := collector.NewGPFSCollector(ctx, cfg)
	if err != nil {
		logrus.WithField("component", "gpfs").Errorf("NewGpfsComponent create collector failed: %v", err)
		return nil, err
	}

	checkers, err := checker.NewCheckers(ctx, cfg)
	if err != nil {
		logrus.WithField("component", "gpfs").Errorf("NewGpfsComponent create checkers failed: %v", err)
		return nil, err
	}

	component := &component{
		ctx:         ctx,
		cancel:      cancel,
		collector:   collector,
		checkers:    checkers,
		cfg:         cfg,
		cacheBuffer: make([]*common.Result, cfg.CacheSize),
		cacheInfo:   make([]common.Info, cfg.CacheSize),
		cacheSize:   cfg.CacheSize,
	}
	service := common.NewCommonService(ctx, cfg, component.HealthCheck)
	component.service = service

	return component, nil
}

func (c *component) Name() string {
	return commonCfg.ComponentNameGpfs
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	cctx, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()
	info, err := c.collector.Collect(cctx)
	if err != nil {
		logrus.WithField("component", "gpfs").Errorf("failed to collect gpfs info: %v", err)
		return nil, err
	}
	gpfs_info, ok := info.(*collector.GPFSInfo)
	if !ok {
		logrus.WithField("component", "gpfs").Errorf("wrong gpfs collector info type")
		return nil, err
	}

	status := commonCfg.StatusNormal
	level := commonCfg.LevelInfo
	checker_results := make([]*common.CheckerResult, 0)
	for _, checker := range c.checkers {
		check_result, err := checker.Check(cctx, gpfs_info.FilterResults[checker.Name()])
		if err != nil {
			logrus.WithField("component", "gpfs").Errorf("failed to check: %v", err)
			continue
		}
		checker_results = append(checker_results, check_result)
		if check_result.Level == commonCfg.LevelCritical && check_result.Status == commonCfg.StatusAbnormal {
			status = commonCfg.StatusAbnormal
			level = check_result.Level
		}
	}

	resResult := &common.Result{
		Item:       commonCfg.ComponentNameGpfs,
		Status:     status,
		Level:      level,
		Suggestion: string("Repaire and Reboot the system"),
		Checkers:   checker_results,
		Time:       time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = resResult
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if resResult.Status == commonCfg.StatusAbnormal {
		logrus.WithField("component", "gpfs").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "gpfs").Infof("Health Check PASSED")
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
	config, ok := cfg.(*config.GpfsConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for gpfs")
	}
	c.cfg = config
	c.cfgMutex.Unlock()
	return c.service.Update(ctx, cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}
