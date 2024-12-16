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
	"github.com/scitix/sichek/components/infiniband/config"
	commonCfg "github.com/scitix/sichek/config"

	"github.com/sirupsen/logrus"
)

var (
	infinibandComponent     *component
	infinibandComponentOnce sync.Once
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc
	spec   *config.InfinibandHCASpec
	info   common.Info

	cfg      *config.InfinibandConfig
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

func NewInfinibandComponent(cfgFile string) (comp common.Component, err error) {
	infinibandComponentOnce.Do(func() {
		infinibandComponent, err = newInfinibandComponent(cfgFile)
		if err != nil {
			panic(err)
		}
	})
	return infinibandComponent, nil
}

func newInfinibandComponent(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.InfinibandConfig{}
	if len(cfgFile) == 0 {
		cfg, err = config.DefaultConfig()
		if err != nil {
			logrus.WithField("component", "infiniband").Error("Infiniband use default config failed ", err)
			return nil, err
		}
	} else {
		err = cfg.LoadFromYaml(cfgFile)
		if err != nil {
			logrus.WithField("component", "infiniband").Error(err)
		}
	}
	logrus.WithField("component", "infiniband").Infof("checker items:%v", cfg.Checkers)

	var IBSpecCfg config.InfinibandHCASpec
	specCfg, err := IBSpecCfg.GetHCASpec()
	if err != nil {
		logrus.WithField("component", "infiniband").Errorf("fail to get the ib spec cfg, err:%v", err)
	}

	checkerConstructors := map[string]func(*config.InfinibandHCASpec) (common.Checker, error){
		config.ChekIBOFED:         checker.NewIBOFEDChecker,
		config.ChekIBFW:           checker.NewFirmwareChecker,
		config.ChekIBState:        checker.NewIBStateChecker,
		config.ChekIBPhyState:     checker.NewIBPhyStateChecker,
		config.CheckPCIEACS:       checker.NewPCIEACSChecker,
		config.CheckPCIEMRR:       checker.NewPCIEMRRChecker,
		config.CheckPCIESpeed:     checker.NewIBPCIESpeedChecker,
		config.CheckPCIEWidth:     checker.NewIBPCIEWidthChecker,
		config.CheckPCIETreeSpeed: checker.NewBPCIETreeSpeedChecker,
		config.CheckPCIETreeWidth: checker.NewIBPCIETreeWidthChecker,
		config.CheckIBKmod:        checker.NewIBKmodChecker,
		config.ChekIBPortSpeed:    checker.NewIBPortSpeedChecker,
		config.CheckIBDevs:        checker.NewIBDevsChecker,
	}

	checkers := make([]common.Checker, 0)
	checkerIndex := 0
	ignoreChecker := 0
	for _, checkItem := range cfg.Checkers {
		if constructor, exists := checkerConstructors[checkItem]; exists {
			chk, err := constructor(specCfg)
			if err != nil {
				logrus.WithField("component", "infiniband").Errorf("fail create the checker:%s error:%v", checkItem, err)
				continue
			}
			checkers = append(checkers, chk)
			checkerIndex += 1
			logrus.WithField("component", "infiniband").Infof("create checker:%2d :%s", checkerIndex, checkItem)
		} else {
			ignoreChecker += 1
			logrus.WithField("component", "infiniband").Warnf("ignore chcker:%d : %s", ignoreChecker, checkItem)
		}
	}

	var info collector.InfinibandInfo
	var spec config.InfinibandHCASpec
	ibSpec, err := spec.GetHCASpec()
	if err != nil {
		logrus.WithField("component", "infiniband").Infof("fail to get the ib spec err %v", err)
	}

	component := &component{
		ctx:         ctx,
		cancel:      cancel,
		spec:        ibSpec,
		info:        info.GetIBInfo(),
		checkers:    checkers,
		cfg:         cfg,
		cacheBuffer: make([]*common.Result, cfg.GetCacheSize()),
		cacheInfo:   make([]common.Info, cfg.GetCacheSize()),
		currIndex:   0,
		cacheSize:   cfg.GetCacheSize(),
	}
	component.service = common.NewCommonService(ctx, cfg, component.HealthCheck)
	return component, nil
}

func (c *component) Name() string {
	return commonCfg.ComponentNameInfiniband
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	cctx, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()

	InfinibandInfo, ok := c.info.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("expected c.info to be of type *collector.InfinibandInfo, got %T", c.info)
	}

	status := commonCfg.StatusNormal
	level := commonCfg.LevelInfo

	checkerResults := make([]*common.CheckerResult, 0)

	// var err error

	for _, checker := range c.checkers {
		logrus.WithField("component", "infiniband").Debugf("do the check: %s", checker.Name())
		result, err := checker.Check(cctx, InfinibandInfo)
		if err != nil {
			logrus.WithField("component", "infiniband").Errorf("failed to check: %v", err)
			continue
		}
		checkerResults = append(checkerResults, result)
	}

	for _, checkItem := range checkerResults {
		logrus.WithField("component", "infiniband").Infof("check Item:%s, status:%s, level:%s\n", checkItem.Name, status, level)
	}

	for _, checkItem := range checkerResults {
		if checkItem.Status == commonCfg.StatusAbnormal {
			status = commonCfg.StatusAbnormal
			level = config.InfinibandCheckItems[checkItem.Name].Level
			logrus.WithField("component", "infiniband").Errorf("check Item:%s, status:%s, level:%s\n", checkItem.Name, status, level)
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

	finalResult := &common.Result{
		Item:     commonCfg.ComponentNameInfiniband,
		Node:     hostname,
		Status:   status,
		Level:    level,
		RawData:  info,
		Checkers: checkerResults,
		Time:     time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = InfinibandInfo
	c.cacheBuffer[c.currIndex] = finalResult
	c.currIndex = (c.currIndex + 1) % c.cfg.GetCacheSize()
	c.cacheMtx.Unlock()

	return finalResult, nil
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
	config, ok := cfg.(*config.InfinibandConfig)
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
