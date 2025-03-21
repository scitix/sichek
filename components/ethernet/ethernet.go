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
package ethernet

import (
	"context"
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/checker"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"

	"sync"
	"time"

	commonCfg "github.com/scitix/sichek/config"

	"github.com/sirupsen/logrus"
)

var (
	ethernetComponent     *component
	ethernetComponentOnce sync.Once
)

type component struct {
	ctx    context.Context
	cancel context.CancelFunc
	spec   *config.EthernetSpec
	info   common.Info

	cfg      *config.EthernetConfig
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

func NewEthernetComponent(cfgFile string) (comp common.Component, err error) {
	ethernetComponentOnce.Do(func() {
		ethernetComponent, err = newEthernetComponent(cfgFile)
		if err != nil {
			panic(err)
		}
	})
	return ethernetComponent, nil
}

func newEthernetComponent(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.EthernetConfig{}
	if len(cfgFile) == 0 {
		cfg, err = config.DefaultConfig()
		if err != nil {
			logrus.WithField("component", "ethernet").Error("ethernet use default config failed ", err)
			return nil, err
		}
	} else {
		err = cfg.LoadFromYaml(cfgFile)
		if err != nil {
			logrus.WithField("component", "ethernet").Error(err)
		}
	}
	logrus.WithField("component", "ethernet").Infof("checker items:%v", cfg.Cherkers)

	var EthSpecCfg config.EthernetSpec
	specCfg, err := EthSpecCfg.GetEthSpec()
	if err != nil {
		logrus.WithField("component", "ethernet").Errorf("fail to get the ib spec cfg, err:%v", err)
	}

	checkers := make([]common.Checker, 0)
	checkerIndex := 0
	for _, checkItem := range cfg.Cherkers {
		switch checkItem {
		case "phy_state":
			checkerIndex = checkerIndex + 1
			checker, err := checker.NewEthPhyStateChecker(specCfg)
			if err != nil {
				logrus.WithField("component", "ethernet").Errorf("Fail to create the checker: %d, err: %v", checkerIndex, err)
			}
			checkers = append(checkers, checker)
			logrus.WithField("component", "ethernet").Infof("create the checker %d: %s", checkerIndex, checkItem)
		}
	}

	var info collector.EthernetInfo
	var spec config.EthernetSpec
	ethSpec, err := spec.GetEthSpec()
	if err != nil {
		logrus.WithField("component", "ethernet").Infof("fail to get the ib spec err %v", err)
	}

	component := &component{
		ctx:         ctx,
		cancel:      cancel,
		spec:        ethSpec,
		info:        info.GetEthInfo(),
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

	ethernetInfo, ok := c.info.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("expected c.info to be of type *collector.EthernetInfo, got %T", c.info)
	}

	status := commonCfg.StatusNormal
	checkerResults := make([]*common.CheckerResult, 0)
	var err error
	var level string = commonCfg.LevelInfo

	for _, cherker := range c.checkers {
		logrus.WithField("component", "ethernet").Debugf("do the check: %s", cherker.Name())
		result, err := cherker.Check(cctx, ethernetInfo)
		if err != nil {
			logrus.WithField("component", "ethernet").Errorf("failed to check: %v", err)
			continue
		}
		checkerResults = append(checkerResults, result)
	}

	for _, checkItem := range checkerResults {
		logrus.WithField("component", "ethernet").Infof("check Item:%s, status:%s, level:%s", checkItem.Name, status, level)
	}

	for _, checkItem := range checkerResults {
		if checkItem.Status == commonCfg.StatusAbnormal {
			status = commonCfg.StatusAbnormal
			level = config.EthCheckItems[checkItem.Name].Level
			logrus.WithField("component", "ethernet").Errorf("check Item:%s, status:%s, level:%s", checkItem.Name, status, level)
			break
		}
	}

	info, err := ethernetInfo.JSON()
	if err != nil {
		return nil, err
	}

	finalResult := &common.Result{
		Item:     commonCfg.ComponentNameInfiniband,
		Status:   status,
		Level:    level,
		RawData:  info,
		Checkers: checkerResults,
		Time:     time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = ethernetInfo
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

func (c *component) Update(ctx context.Context, cfg common.ComponentConfig) error {
	c.cfgMutex.Lock()
	config, ok := cfg.(*config.EthernetConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for infiniband")
	}
	c.cfg = config
	c.cfgMutex.Unlock()
	return c.service.Update(ctx, cfg)
}

func (c *component) Start(ctx context.Context) <-chan *common.Result {
	return c.service.Start(ctx)
}

func (c *component) Status() bool {
	return c.service.Status()
}

func (c *component) Stop() error {
	return c.service.Stop()

}
