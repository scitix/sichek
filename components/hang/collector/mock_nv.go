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
package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/components/nvidia/config"
	"github.com/scitix/sichek/consts"
)

type component struct {
	name   string
	ctx    context.Context
	cancel context.CancelFunc

	cfg      *config.NvidiaUserConfig
	cfgMutex sync.RWMutex

	nvmlInst nvml.Interface
	// collector collector.NvidiaInfo
	checkers []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	serviceMtx    sync.RWMutex
	running       bool
	resultChannel chan *common.Result
}

var (
	nvidiaComponent     *component
	nvidiaComponentOnce sync.Once
)

func NewMockNvidiaComponent(cfgFile string) (common.Component, error) {
	var err error
	nvidiaComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component hang: %v", r)
			}
		}()
		nvidiaComponent, err = newMockNvidia(cfgFile)
	})
	now = time.Now()
	return nvidiaComponent, err
}

func newMockNvidia(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	cfg := &config.NvidiaUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Nvidia == nil {
		return nil, fmt.Errorf("NewMockNvidiaComponent get user config failed")
	}

	component := &component{
		name:   consts.ComponentNameNvidia,
		ctx:    ctx,
		cancel: cancel,

		cfg:         cfg,
		cfgMutex:    sync.RWMutex{},
		nvmlInst:    nil,
		checkers:    nil,
		cacheMtx:    sync.RWMutex{},
		cacheBuffer: make([]*common.Result, cfg.Nvidia.CacheSize),
		cacheInfo:   make([]common.Info, cfg.Nvidia.CacheSize),
		currIndex:   0,
		cacheSize:   cfg.Nvidia.CacheSize,
		running:     false,
	}
	return component, nil
}

func (c *component) Name() string {
	return c.name
}

var now time.Time

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	now = now.Add(10 * time.Second)
	nvdiaCollector := &collector.NvidiaInfo{}
	nvdiaCollector.Time = now
	nvdiaCollector.DeviceCount = 8
	for i := 0; i < 8; i++ {
		deviceInfo := collector.DeviceInfo{}
		deviceInfo.Index = i
		deviceInfo.UUID = fmt.Sprintf("mock-uuid-%d", i)
		deviceInfo.Power.PowerUsage = 75
		deviceInfo.Utilization.MemoryUsagePercent = 0
		deviceInfo.Utilization.GPUUsagePercent = 100
		deviceInfo.Power.PowerViolations = 0
		deviceInfo.PCIeInfo.PCIeRx = 0
		deviceInfo.PCIeInfo.PCIeTx = 0
		nvdiaCollector.DevicesInfo = append(nvdiaCollector.DevicesInfo, deviceInfo)
	}

	result := &common.Result{
		Item:     consts.ComponentNameNvidia,
		Status:   consts.StatusAbnormal,
		Checkers: make([]*common.CheckerResult, 0),
		Time:     time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = nil
	c.cacheInfo[c.currIndex] = nvdiaCollector
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	return result, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
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

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

func (c *component) Start() <-chan *common.Result {
	c.serviceMtx.Lock()
	if c.running {
		c.serviceMtx.Unlock()
		return c.resultChannel
	}
	c.running = true
	c.serviceMtx.Unlock()

	go func() {
		ticker := time.NewTicker(c.cfg.GetQueryInterval().Duration)
		defer ticker.Stop()

		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.serviceMtx.Lock()
				result, err := c.HealthCheck(c.ctx)
				c.serviceMtx.Unlock()
				if err != nil {
					fmt.Printf("%s analyze failed: %v\n", c.name, err)
					continue
				}
				if result.Level == consts.LevelCritical || result.Level == consts.LevelWarning {
					c.serviceMtx.Lock()
					c.resultChannel <- result
					c.serviceMtx.Unlock()
				}
			}
		}
	}()

	c.serviceMtx.Lock()
	c.running = true
	c.serviceMtx.Unlock()
	return c.resultChannel
}

func (c *component) Stop() error {
	c.cancel()
	c.serviceMtx.Lock()
	close(c.resultChannel)
	c.running = false
	c.serviceMtx.Unlock()

	return nil
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	nvCfg, ok := cfg.(*config.NvidiaUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for nvidia")
	}
	c.cfg = nvCfg
	c.cfgMutex.Unlock()
	return nil
}

func (c *component) Status() bool {
	c.serviceMtx.RLock()
	defer c.serviceMtx.RUnlock()

	return c.running
}

func (c *component) GetTimeout() time.Duration {
	return c.cfg.GetQueryInterval().Duration
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	return true
}
