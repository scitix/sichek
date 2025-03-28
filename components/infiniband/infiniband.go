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
package infiniband

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/checker"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/config"
	"github.com/scitix/sichek/config/hca"
	"github.com/scitix/sichek/config/infiniband"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

var (
	infinibandComponent     *component
	infinibandComponentOnce sync.Once
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc
	spec   *infiniband.InfinibandSpecItem
	info   common.Info

	cfg      *infiniband.InfinibandConfig
	cfgMutex sync.RWMutex

	// collector common.Collector
	checkers []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

func NewInfinibandComponent(componentConfig *config.ComponentConfig, ignoredCheckers []string) (comp common.Component, err error) {
	infinibandComponentOnce.Do(func() {
		infinibandComponent, err = newInfinibandComponent(componentConfig, ignoredCheckers)
		if err != nil {
			panic(err)
		}
	})
	return infinibandComponent, nil
}

func newInfinibandComponent(componentConfig *config.ComponentConfig, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	// step1: load user define check item
	cfg, ibSpec, err := GetConfig(componentConfig)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("NewComponent get config failed: %v", err)
		return nil, err
	}
	if len(ignoredCheckers) > 0 {
		cfg.IgnoredCheckers = ignoredCheckers
	}

	checkers, err := checker.NewCheckers(cfg, ibSpec)
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("NewCheckers failed: %v", err)
		return nil, err
	}

	var info collector.InfinibandInfo

	component := &component{
		ctx:         ctx,
		cancel:      cancel,
		spec:        ibSpec,
		info:        info.GetIBInfo(),
		checkers:    checkers,
		cfg:         cfg,
		cacheBuffer: make([]*common.Result, cfg.CacheSize),
		cacheInfo:   make([]common.Info, cfg.CacheSize),
		currIndex:   0,
		cacheSize:   cfg.CacheSize,
	}
	// step4: start the service
	component.service = common.NewCommonService(ctx, cfg, component.HealthCheck)

	// step5: return the component
	return component, nil
}

func (c *component) Name() string {
	return consts.ComponentNameInfiniband
}

func GetConfig(componentConfig *config.ComponentConfig) (*infiniband.InfinibandConfig, *infiniband.InfinibandSpecItem, error) {
	cfg, specCfg := componentConfig.GetConfigByComponentName(consts.ComponentNameInfiniband)
	if cfg == nil || specCfg == nil {
		logrus.WithField("component", "infiniband").Errorf("NewComponent get config cfg: %v, specCfg: %v", cfg, specCfg)
		return nil, nil, fmt.Errorf("NewComponent get config failed")
	}
	infinibandCfg, ok := cfg.(*infiniband.InfinibandConfig)
	if !ok {
		return nil, nil, fmt.Errorf("invalid basic config type for infiniband component")
	}
	infinibandSpecCfgs, ok := specCfg.(*infiniband.InfinibandSpec)
	if !ok {
		return nil, nil, fmt.Errorf("invalid spec config type for infiniband component")
	}
	_, hcaSpec := componentConfig.GetConfigByComponentName(consts.ComponentNameHCA)
	hcaSpecCfg, ok := hcaSpec.(*hca.HCASpec)
	if !ok || hcaSpecCfg == nil {
		return nil, nil, fmt.Errorf("11212invalid hca spec type for infiniband component")
	}
	infinibandSpecCfgs.LoadHCASpec(hcaSpecCfg)
	infinibandSpecCfg, err := infinibandSpecCfgs.GetClusterInfinibandSpec()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster infiniband spec: %v", err)
	}
	return infinibandCfg, infinibandSpecCfg, nil
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	cctx, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()

	InfinibandInfo, ok := c.info.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("expected c.info to be of type *collector.InfinibandInfo, got %T", c.info)
	}
	status := consts.StatusNormal

	checkerResults := make([]*common.CheckerResult, 0)
	var level string = consts.LevelInfo
	var err error

	for _, cherker := range c.checkers {
		logrus.WithField("component", "infiniband").Debugf("do the check: %s", cherker.Name())
		result, err := cherker.Check(cctx, InfinibandInfo)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("failed to check: %v", err)
			continue
		}
		checkerResults = append(checkerResults, result)
	}

	for _, checkItem := range checkerResults {
		if checkItem.Status == consts.StatusAbnormal {
			logrus.WithField("component", "infiniband").Warnf("check Item:%s, status:%s, level:%s", checkItem.Name, status, level)
		}
	}

	for _, checkItem := range checkerResults {
		if checkItem.Status == consts.StatusAbnormal {
			status = consts.StatusAbnormal
			level = infiniband.InfinibandCheckItems[checkItem.Name].Level
			logrus.WithField("component", "infiniband").Errorf("check Item:%s, status:%s, level:%s", checkItem.Name, status, level)
			break
		}
	}
	info, err := InfinibandInfo.JSON()
	if err != nil {
		return nil, fmt.Errorf("failed to convert infiniband info to JSON: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("failed to get the hostname: %v", err)
		hostname = "unknown"
	}

	resResult := &common.Result{
		Item:     consts.ComponentNameInfiniband,
		Node:     hostname,
		Status:   status,
		Level:    level,
		RawData:  info,
		Checkers: checkerResults,
		Time:     time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = InfinibandInfo
	c.cacheBuffer[c.currIndex] = resResult
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	if resResult.Status == consts.StatusAbnormal {
		logrus.WithField("component", "Infiniband").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "Infiniband").Infof("Health Check PASSED")
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

// 更新组件的配置信息，同时更新service
func (c *component) Update(ctx context.Context, cfg common.ComponentConfig) error {
	c.cfgMutex.Lock()
	config, ok := cfg.(*infiniband.InfinibandConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for infiniband")
	}
	c.cfg = config
	c.cfgMutex.Unlock()
	return c.service.Update(ctx, cfg)
}

// Start方法用于systemD的启动，周期性地执行HealthCheck函数获取数据，并将结果发送到resultChannel
func (c *component) Start(ctx context.Context) <-chan *common.Result {
	return c.service.Start(ctx)
}

// 返回组件的运行情况
func (c *component) Status() bool {
	return c.service.Status()
}

// 用于systemD的停止
func (c *component) Stop() error {
	return c.service.Stop()

}
