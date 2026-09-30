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
package gpuprobe

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/gpuprobe/checker"
	"github.com/scitix/sichek/components/gpuprobe/collector"
	"github.com/scitix/sichek/components/gpuprobe/config"
	"github.com/scitix/sichek/components/gpuprobe/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

const componentName = "gpuprobe"

// compile-time check that *component satisfies the Component interface
var _ common.Component = (*component)(nil)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string

	cfg      *config.GpuProbeUserConfig
	cfgMutex sync.Mutex

	collector *collector.Collector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
	metrics *metrics.GpuProbeMetrics
}

var (
	gpuProbeComponent     *component
	gpuProbeComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	gpuProbeComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component gpuprobe: %v", r)
			}
		}()
		gpuProbeComponent, err = newComponent(cfgFile, specFile)
	})
	return gpuProbeComponent, err
}

func newComponent(cfgFile string, specFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	cfg := &config.GpuProbeUserConfig{}
	if e := common.LoadUserConfig(cfgFile, cfg); e != nil || cfg.GpuProbe == nil {
		return nil, fmt.Errorf("NewComponent gpuprobe load user config failed: %v", e)
	}
	spec, err := config.LoadSpec(specFile)
	if err != nil {
		return nil, err
	}
	checkerPointer, err := checker.NewGpuProbeChecker(spec)
	if err != nil {
		return nil, err
	}
	var m *metrics.GpuProbeMetrics
	if cfg.GpuProbe.EnableMetrics {
		m = metrics.NewGpuProbeMetrics()
	}
	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: componentName,
		collector:     collector.NewGpuProbeCollector(spec),
		checkers:      []common.Checker{checkerPointer},
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cfg.GpuProbe.CacheSize),
		cacheInfo:     make([]common.Info, cfg.GpuProbe.CacheSize),
		cacheSize:     cfg.GpuProbe.CacheSize,
		metrics:       m,
	}
	comp.service = common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	return comp, nil
}

func (c *component) Name() string { return c.componentName }

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "gpuprobe").Errorf("%v", err)
		return nil, err
	}
	gpuInfo, ok := info.(*collector.GpuProbeInfo)
	if !ok {
		return nil, fmt.Errorf("wrong gpuprobe info type")
	}
	if c.cfg.GpuProbe.EnableMetrics && c.metrics != nil {
		c.metrics.ExportMetrics(gpuInfo)
	}
	result := common.Check(ctx, c.Name(), gpuInfo, c.checkers)

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "gpuprobe").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "gpuprobe").Infof("Health Check PASSED")
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
	if c.currIndex == 0 {
		return c.cacheInfo[c.cacheSize-1], nil
	}
	return c.cacheInfo[c.currIndex-1], nil
}

func (c *component) Start() <-chan *common.Result { return c.service.Start() }
func (c *component) Stop() error                  { return c.service.Stop() }
func (c *component) Status() bool                 { return c.service.Status() }
func (c *component) GetTimeout() time.Duration    { return c.cfg.GetQueryInterval().Duration }

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	p, ok := cfg.(*config.GpuProbeUserConfig)
	if !ok {
		c.cfgMutex.Unlock()
		return fmt.Errorf("update wrong config type for gpuprobe")
	}
	c.cfg = p
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	allPassed := result.Status == consts.StatusNormal
	for _, r := range result.Checkers {
		color := consts.Green
		if r.Status != consts.StatusNormal {
			color = consts.LevelColor(r.Level)
		}
		if summaryPrint {
			fmt.Printf("GPUProbe: %s%s%s %s\n", color, r.Status, consts.Reset, r.Detail)
		}
	}
	return allPassed
}
