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
package nvidia

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/checker"
	"github.com/scitix/sichek/components/nvidia/collector"
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
	collector *collector.NvidiaCollector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	xidPoller *XidEventPoller

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
		logrus.WithField("component", "NVIDIA").Errorf("failed to initialize NVML: %v", nvml.ErrorString(ret))
		return nil, fmt.Errorf("%v", nvml.ErrorString(ret))
	}
	return nvmlInst, nil
}

/*
  - TODO: reinit nvmlInst becomes invalid during the program's runtime
    Key NVML Error Codes to Handle
    nvml.ERROR_UNINITIALIZED: NVML was not initialized or nvml.Shutdown was called.
    nvml.ERROR_DRIVER_NOT_LOADED: NVIDIA driver is not running or not installed.
    nvml.ERROR_NO_PERMISSION: Lack of access to the GPU.
    nvml.ERROR_GPU_IS_LOST: GPU is inaccessible, possibly due to a hardware failure.
    nvml.ERROR_UNKNOWN: An unexpected issue occurred
    If nvmlInst becomes invalid, reinitialize it.
*/
func ReNewNvml(c *component) error {
	shutdownRet := c.nvmlInst.Shutdown()
	if shutdownRet != nvml.SUCCESS {
		logrus.WithField("component", "nvidia").Errorf("failed to shutdown NVML: %v", shutdownRet.Error())
		return fmt.Errorf("shutdownRet:%s", shutdownRet.Error())
	}

	nvmlInst, ret := NewNvml()
	if ret != nil {
		logrus.WithField("component", "NVIDIA").Errorf("failed to Reinitialize NVML: %v", ret)
		return ret
	}
	c.nvmlInst = nvmlInst
	return nil
}

func StopNvml(nvmlInst nvml.Interface) error {
	ret := nvmlInst.Shutdown()
	if ret != nvml.SUCCESS {
		logrus.WithField("component", "NVIDIA").Errorf("failed to shutdown NVML: %v", nvml.ErrorString(ret))
		return ret
	}
	return nil
}

func NewComponent(cfgFile string, ignored_checkers []string) (comp common.Component, err error) {
	nvidiaComponentOnce.Do(func() {
		nvidiaComponent, err = newNvidia(cfgFile, ignored_checkers)
		if err != nil {
			panic(err)
		}
	})
	return nvidiaComponent, nil
}

func getDeviceID(nvmlInst nvml.Interface) (string, error) {
	// In case of GPU error, iterate through all GPUs to find the first valid one
	deviceCount, err := nvmlInst.DeviceGetCount()
	if err != nvml.SUCCESS {
		logrus.WithField("component", "NVIDIA").Errorf("Failed to get device count: %s\n", nvml.ErrorString(err))
		stopErr := StopNvml(nvmlInst)
		if stopErr != nil {
			return "", fmt.Errorf("failed to stop numl: %s", stopErr)
		}
		return "", fmt.Errorf("failed to get device count: %s", nvml.ErrorString(err))
	}
	var deviceID string
	for i := 0; i < deviceCount; i++ {
		device, err := nvmlInst.DeviceGetHandleByIndex(i)
		if err != nvml.SUCCESS {
			logrus.WithField("component", "NVIDIA").Errorf("failed to get Nvidia GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		pciInfo, err := device.GetPciInfo()
		if err != nvml.SUCCESS {
			logrus.WithField("component", "NVIDIA").Errorf("failed to get PCIe Info  for NVIDIA GPU %d: %s", i, nvml.ErrorString(err))
			continue
		}
		deviceID = fmt.Sprintf("0x%x", pciInfo.PciDeviceId)
		return deviceID, nil
	}
	return "", fmt.Errorf("failed to get product name for NVIDIA GPU")
}

func newNvidia(cfgFile string, ignored_checkers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	nvmlInst, err := NewNvml()
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewNvidia create nvml failed: %v", err)
		return nil, err
	}
	cfg := &nvidiaCfg.NvidiaConfig{}
	deviceID, err := getDeviceID(nvmlInst)
	if err != nil {
		panic("failed to get PCIe Device ID for NVIDIA GPU")
	}
	specFile, err := nvidiaCfg.GetSpec(deviceID)
	if err != nil {
		panic("failed to get spec file for NVIDIA GPU")
	}
	err = cfg.LoadFromYaml(cfgFile, specFile)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewNvidia load config yaml %s failed: %v", cfgFile, err)
		return nil, err
	}
	if len(ignored_checkers) > 0 {
		cfg.ComponentConfig.IgnoredCheckers = ignored_checkers
	}

	collector, err := collector.NewNvidiaCollector(nvmlInst, cfg.Spec.GpuNums)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewNvidiaCollector failed: %v", err)
		return nil, err
	}

	checkers, err := checker.NewCheckers(cfg, nvmlInst)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewCheckers failed: %v", err)
		return nil, err
	}

	resultChannel := make(chan *common.Result)
	xidPoller, err := NewXidEventPoller(ctx, cfg.ComponentConfig, nvmlInst, resultChannel)
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("NewXidEventPoller failed: %v", err)
		return nil, err
	}

	component := &component{
		name:          "NVIDIA",
		ctx:           ctx,
		cancel:        cancel,
		cfg:           cfg.ComponentConfig,
		cfgMutex:      sync.RWMutex{},
		nvmlInst:      nvmlInst,
		collector:     collector,
		checkers:      checkers,
		cacheMtx:      sync.RWMutex{},
		cacheBuffer:   make([]*common.Result, cfg.ComponentConfig.CacheSize),
		cacheInfo:     make([]common.Info, cfg.ComponentConfig.CacheSize),
		currIndex:     0,
		cacheSize:     cfg.ComponentConfig.CacheSize,
		xidPoller:     xidPoller,
		running:       false,
		resultChannel: resultChannel,
	}

	return component, nil
}

func (c *component) Name() string {
	return c.name
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	cctx, cancel := context.WithTimeout(ctx, c.cfg.GetQueryInterval()*time.Second)
	defer cancel()

	nvidiaInfo, err := c.collector.Collect()
	if err != nil {
		logrus.WithField("component", "NVIDIA").Errorf("failed to collect nvidia info: %v", err)
		return nil, err
	}

	status := commonCfg.StatusNormal
	level := commonCfg.LevelInfo
	checkerResults := make([]*common.CheckerResult, 0)
	for _, checker := range c.checkers {
		checkResult, err := checker.Check(cctx, nvidiaInfo)
		if err != nil {
			logrus.WithField("component", "NVIDIA").Errorf("failed to check: %v", err)
			// continue
		}
		checkerResults = append(checkerResults, checkResult)

		if checkResult.Status == commonCfg.StatusAbnormal {
			status = commonCfg.StatusAbnormal
			if checkResult.Level == commonCfg.LevelCritical || level == commonCfg.LevelInfo {
				level = checkResult.Level
			}
		}
	}

	for _, checkItem := range checkerResults {
		logrus.WithField("component", "NVIDIA").Infof("check Item:%s, status:%s, level:%s\n", checkItem.Name, status, level)
	}

	for _, checkItem := range checkerResults {
		if checkItem.Status == commonCfg.StatusAbnormal {
			status = commonCfg.StatusAbnormal
			level = fmt.Sprintf("check Item:%s, status:%s, level:%s\n", checkItem.Name, status, level)
			break
		}
	}

	res_result := &common.Result{
		Item:     c.name,
		Status:   status,
		Level:    level,
		Checkers: checkerResults,
		Time:     time.Now(),
	}

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = res_result
	c.cacheInfo[c.currIndex] = nvidiaInfo
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if res_result.Status == commonCfg.StatusAbnormal {
		logrus.WithField("component", "NVIDIA").Errorf("health check failed")
	} else {
		logrus.WithField("component", "NVIDIA").Infof("health check passed")
	}

	return res_result, nil
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

	go func() {
		err := c.xidPoller.Start()
		if err != nil {
			logrus.WithField("component", "NVIDIA").Errorf("start xid poller failed: %v", err)
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
