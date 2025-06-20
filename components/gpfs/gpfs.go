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
	filter "github.com/scitix/sichek/components/common/eventfilter"
	"github.com/scitix/sichek/components/gpfs/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.GpfsUserConfig
	cfgMutex      sync.Mutex

	filter *filter.EventFilter

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

func NewGpfsComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	gpfsComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component gpfs: %v", r)
			}
		}()
		gpfsComponent, err = newGpfsComponent(cfgFile, specFile)
	})
	return gpfsComponent, err
}

func newGpfsComponent(cfgFile string, specFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.GpfsUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Gpfs == nil {
		logrus.WithField("component", "gpfs").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		return nil, fmt.Errorf("NewGpfsComponent get user config failed")
	}
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "gpfs").Errorf("failed to NewComponent: %v", err)
		return nil, err
	}

	filterPointer, err := filter.NewEventFilter(consts.ComponentNameGpfs, eventRules, 1, 100)
	if err != nil {
		return nil, err
	}

	component := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameGpfs,
		filter:        filterPointer,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cfg.Gpfs.CacheSize),
		cacheInfo:     make([]common.Info, cfg.Gpfs.CacheSize),
		cacheSize:     cfg.Gpfs.CacheSize,
	}
	service := common.NewCommonService(ctx, cfg, component.componentName, component.GetTimeout(), component.HealthCheck)
	component.service = service

	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	result := c.filter.Check()
	result.Item = c.componentName
	result.Time = time.Now()
	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "gpfs").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "gpfs").Infof("Health Check PASSED")
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
	var info common.Info
	if c.currIndex == 0 {
		info = c.cacheInfo[c.cacheSize-1]
	} else {
		info = c.cacheInfo[c.currIndex-1]
	}
	return info, nil
}

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.GpfsUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for gpfs")
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
	checkAllPassed := true
	var mountPrint string
	gpfsEvent := make(map[string]string)

	checkerResults := result.Checkers
	for _, result := range checkerResults {
		statusColor := consts.Green
		if result.Status != consts.StatusNormal {
			statusColor = consts.Red
			checkAllPassed = false
			gpfsEvent[result.Name] = fmt.Sprintf("Event: %s%s%s", consts.Red, result.ErrorName, consts.Reset)
		}

		switch result.Name {
		case config.FilesystemUnmountCheckerName:
			mountPrint = fmt.Sprintf("GPFS: %sMounted%s", statusColor, consts.Reset)
		}
	}

	utils.PrintTitle("GPFS", "-")
	fmt.Printf("%s\n", mountPrint)
	for _, v := range gpfsEvent {
		fmt.Printf("\t%s\n", v)
	}
	return checkAllPassed
}
