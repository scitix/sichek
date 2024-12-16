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

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	nvidiaCollector "github.com/scitix/sichek/components/nvidia/collector"
	nvidiaCfg "github.com/scitix/sichek/components/nvidia/config"
	commonCfg "github.com/scitix/sichek/config"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/sirupsen/logrus"
)

type component struct {
	name   string
	ctx    context.Context
	cancel context.CancelFunc

	cfg      nvidiaCfg.ComponentConfig
	cfgMutex sync.RWMutex // 用于更新时的锁

	nvmlInst  nvml.Interface
	collector collector.NvidiaInfo
	checkers  []common.Checker

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

func NewNvml() (nvml.Interface, error) {
	nvmlInst := nvml.New()
	if ret := nvmlInst.Init(); ret != nvml.SUCCESS {
		logrus.WithField("component", "nvidia").Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
		return nil, fmt.Errorf("%v", nvml.ErrorString(ret))
	}
	return nvmlInst, nil
}

func ReNewNvml(c *component) error {
	nvmlInst, ret := NewNvml()
	if ret != nil {
		logrus.WithField("component", "nvidia").Errorf("failed to Reinitialize NVML: %v", ret)
		return ret
	}
	c.nvmlInst = nvmlInst
	return nil
}

func StopNvml(nvmlInst nvml.Interface) error {
	ret := nvmlInst.Shutdown()
	if ret != nvml.SUCCESS {
		logrus.WithField("component", "nvidia").Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
		return ret
	}
	return nil
}

func NewMockNvidiaComponent(cfgFile string, ignored_checkers []string) (comp common.Component, err error) {
	nvidiaComponentOnce.Do(func() {
		nvidiaComponent, err = newMockNvidia(cfgFile, ignored_checkers)
		if err != nil {
			panic(err)
		}
	})
	now = time.Now()
	return nvidiaComponent, nil
}

func newMockNvidia(cfgFile string, ignored_checkers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &nvidiaCfg.NvidiaConfig{}
	err = cfg.LoadFromYaml("", "")
	if err != nil {
		logrus.WithField("component", "nvidia").Errorf("NewMockNvidia load config yaml %s failed: %v", cfgFile, err)
		return nil, err
	}

	component := &component{
		name:        "nvidia",
		ctx:         ctx,
		cancel:      cancel,
		cfgMutex:    sync.RWMutex{},
		nvmlInst:    nil,
		checkers:    nil,
		cacheMtx:    sync.RWMutex{},
		cacheBuffer: make([]*common.Result, cfg.ComponentConfig.CacheSize),
		cacheInfo:   make([]common.Info, cfg.ComponentConfig.CacheSize),
		currIndex:   0,
		cacheSize:   cfg.ComponentConfig.CacheSize,
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
	collector := &nvidiaCollector.NvidiaInfo{}
	collector.Time = now
	collector.DeviceCount = 8
	for i := 0; i < 8; i++ {
		deviceInfo := nvidiaCollector.DeviceInfo{}
		deviceInfo.Index = i
		deviceInfo.UUID = fmt.Sprintf("mock-uuid-%d", i)
		deviceInfo.Power.PowerUsage = 75
		deviceInfo.Utilization.MemoryUsagePercent = 0
		deviceInfo.Utilization.GPUUsagePercent = 100
		deviceInfo.Power.PowerViolations = 0
		deviceInfo.PCIeInfo.PCIeRx = 0
		deviceInfo.PCIeInfo.PCIeTx = 0
		collector.DevicesInfo = append(collector.DevicesInfo, deviceInfo)
	}

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = nil
	c.cacheInfo[c.currIndex] = collector
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	return nil, nil
}

func (c *component) CacheResults(ctx context.Context) ([]*common.Result, error) {
	c.cacheMtx.Lock()
	defer c.cacheMtx.Unlock()
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
	c.serviceMtx.Lock()
	if c.running {
		c.serviceMtx.Unlock()
		return c.resultChannel
	}
	c.running = true
	c.serviceMtx.Unlock()

	go func() {
		ticker := time.NewTicker(c.cfg.GetQueryInterval() * time.Second)
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
				if result.Level == commonCfg.LevelCritical || result.Level == commonCfg.LevelWarning {
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

func (c *component) Update(ctx context.Context, cfg common.ComponentConfig) error {
	c.cfgMutex.Lock()
	config, ok := cfg.(*nvidiaCfg.ComponentConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for nvidia")
	}
	c.cfg = *config
	c.cfgMutex.Unlock()
	return nil
}

func (c *component) Status() bool {
	c.serviceMtx.RLock()
	defer c.serviceMtx.RUnlock()

	return c.running
}
