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
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc

	cfg      *config.HangUserConfig
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
	hangComponent     common.Component
	hangComponentOnce sync.Once
)

func NewComponent(cfgFile string) (comp common.Component, err error) {
	hangComponentOnce.Do(func() {
		hangComponent, err = newComponent(cfgFile)
		if err != nil {
			panic(err)
		}
	})
	return hangComponent, nil
}

func newComponent(cfgFile string) (comp common.Component, err error) {
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

	var hangCollector common.Collector
	if hangCfg.Hang.NVSMI {
		hangCollector, err = collector.NewHangCollector(ctx, hangCfg)
		if err != nil {
			logrus.WithField("component", "hang").WithError(err).Error("failed to create HangCollector")
			return nil, err
		}
	} else {
		hangCollector, err = collector.NewHangGetter(ctx, hangCfg)
		if err != nil {
			logrus.WithField("component", "hang").WithError(err).Error("failed to create HangCollector")
			return nil, err
		}
	}

	checker := checker.NewHangChecker(hangCfg)

	component := &component{
		ctx:    ctx,
		cancel: cancel,

		cfg: hangCfg,

		collector: hangCollector,
		checker:   checker,

		cacheResultBuffer: make([]*common.Result, hangCfg.Hang.CacheSize),
		cacheInfoBuffer:   make([]common.Info, hangCfg.Hang.CacheSize),
		currIndex:         0,
		cacheSize:         hangCfg.Hang.CacheSize,
	}
	component.service = common.NewCommonService(ctx, hangCfg, component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return consts.ComponentNameHang
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	cctx, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()
	info, err := c.collector.Collect(cctx)
	if err != nil {
		logrus.WithField("component", "hang").WithError(err).Error("failed to Collect()")
		return &common.Result{}, err
	}

	checkRes, err := c.checker.Check(c.ctx, info)
	if err != nil {
		logrus.WithField("component", "hang").WithError(err).Error("failed to Check()")
		return &common.Result{}, err
	}

	resResult := &common.Result{
		Item:       consts.ComponentNameHang,
		Node:       "hang",
		Status:     checkRes.Status,
		Level:      consts.LevelFatal,
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

func (c *component) CacheResults(ctx context.Context) ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
	return c.cacheResultBuffer, nil
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

func (c *component) CacheInfos(ctx context.Context) ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfoBuffer, nil
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

func (c *component) Update(ctx context.Context, cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	hangCfg, ok := cfg.(*config.HangUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for hang")
	}
	c.cfg = hangCfg
	c.cfgMutex.Unlock()
	return c.service.Update(ctx, cfg)
}

func (c *component) Status() bool {
	return c.service.Status()
}
