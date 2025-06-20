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
package cpu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu/checker"
	"github.com/scitix/sichek/components/cpu/collector"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/components/cpu/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string

	cfg      *config.CpuUserConfig
	cfgMutex sync.Mutex

	collector common.Collector
	checkers  []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService

	metrics *metrics.CpuMetrics
}

var (
	cpuComponent     *component
	cpuComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	cpuComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component cpu: %v", r)
			}
		}()
		cpuComponent, err = newComponent(cfgFile, specFile)
	})
	return cpuComponent, err
}

func newComponent(cfgFile string, specFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	cfg := &config.CpuUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.CPU == nil {
		logrus.WithField("component", "cpu").Errorf("NewComponent load config failed or user config is nil, err: %v", err)
		return nil, fmt.Errorf("NewCpuComponent get user config failed")
	}
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("failed to NewComponent: %v", err)
		return nil, err
	}
	collectorPointer, err := collector.NewCpuCollector(ctx, eventRules)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("NewComponent create collector failed: %v", err)
		return nil, err
	}

	checkers, err := checker.NewCheckers(eventRules)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("NewComponent create checkers failed: %v", err)
		return nil, err
	}
	var cpuMetrics *metrics.CpuMetrics
	if cfg.CPU.EnableMetrics {
		cpuMetrics = metrics.NewCpuMetrics()
	}
	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameCPU,
		collector:     collectorPointer,
		checkers:      checkers,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cfg.CPU.CacheSize),
		cacheInfo:     make([]common.Info, cfg.CPU.CacheSize),
		cacheSize:     cfg.CPU.CacheSize,
		metrics:       cpuMetrics,
	}
	service := common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	comp.service = service

	return
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "cpu").Errorf("%v", err)
		return nil, err
	}
	cpuInfo, ok := info.(*collector.CPUOutput)
	if !ok {
		logrus.WithField("component", "cpu").Errorf("wrong cpu info type")
		return nil, err
	}
	if c.cfg.CPU.EnableMetrics {
		c.metrics.ExportMetrics(cpuInfo)
	}
	result := common.Check(ctx, c.Name(), cpuInfo, c.checkers)

	c.cacheMtx.Lock()
	c.cacheInfo[c.currIndex] = info
	c.cacheBuffer[c.currIndex] = result
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()
	if result.Status == consts.StatusAbnormal {
		logrus.WithField("component", "cpu").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "cpu").Infof("Health Check PASSED")
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

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.CpuUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for cpu")
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
	cpuInfo, ok := info.(*collector.CPUOutput)
	if !ok {
		logrus.WithField("component", "cpu").Errorf("invalid data type, expected CPUOutput")
		return false
	}
	osPrint := fmt.Sprintf("OS: %s%s(%s)%s", consts.Green, cpuInfo.HostInfo.OSVersion, cpuInfo.HostInfo.KernelVersion, consts.Reset)
	modelNamePrint := fmt.Sprintf("ModelName: %s%s%s", consts.Green, cpuInfo.CPUArchInfo.ModelName, consts.Reset)
	uptimePrint := fmt.Sprintf("Uptime: %s%s%s", consts.Green, cpuInfo.Uptime, consts.Reset)

	nuamNodes := make([]string, 0, len(cpuInfo.CPUArchInfo.NumaNodeInfo))
	for _, node := range cpuInfo.CPUArchInfo.NumaNodeInfo {
		numaNode := fmt.Sprintf("NUMA node%d CPU(s): %s%s%s", node.ID, consts.Green, node.CPUs, consts.Reset)
		nuamNodes = append(nuamNodes, numaNode)
	}
	taskPrint := fmt.Sprintf("Tasks: %d, %s%d%s thr; %d running", cpuInfo.UsageInfo.SystemProcessesTotal, consts.Green, cpuInfo.UsageInfo.TotalThreadCount, consts.Reset, cpuInfo.UsageInfo.RunnableTaskCount)
	loadAvgPrint := fmt.Sprintf("Load average: %s%.2f %.2f %.2f%s", consts.Green, cpuInfo.UsageInfo.CpuLoadAvg1m, cpuInfo.UsageInfo.CpuLoadAvg5m, cpuInfo.UsageInfo.CpuLoadAvg15m, consts.Reset)
	var performanceModePrint string
	checkerResults := result.Checkers
	for _, result := range checkerResults {
		statusColor := consts.Green
		if result.Status != consts.StatusNormal {
			statusColor = consts.Red
			checkAllPassed = false
		}
		switch result.Name {
		case checker.CPUPerfCheckerName:
			performanceModePrint = fmt.Sprintf("PerformanceMode: %s%s%s", statusColor, result.Curr, consts.Reset)
		}
	}
	if summaryPrint {
		fmt.Printf("\nHostname: %s\n\n", cpuInfo.HostInfo.Hostname)
		utils.PrintTitle("System", "-")
		termWidth, err := utils.GetTerminalWidth()
		printInterval := 40
		if err == nil {
			printInterval = termWidth / 3
		}
		fmt.Printf("%-*s%-*s\n", printInterval, osPrint, printInterval, modelNamePrint)
		fmt.Printf("%-*s%-*s%-*s\n", printInterval, uptimePrint, printInterval, nuamNodes[0], printInterval, taskPrint)
		if cpuInfo.CPUArchInfo.NumaNum > 1 {
			fmt.Printf("%-*s%-*s%-*s\n", printInterval, consts.Green+""+consts.Reset, printInterval, nuamNodes[1], printInterval, loadAvgPrint)
		} else {
			fmt.Printf("%-*s%-*s%-*s\n", printInterval, consts.Green+""+consts.Reset, printInterval, consts.Green+""+consts.Reset, printInterval, loadAvgPrint)
		}
		// TODO: more numa node
		fmt.Printf("%-*s%-*s\n", printInterval, consts.Green+""+consts.Reset, printInterval, performanceModePrint)
		fmt.Println()
	}
	return checkAllPassed
}
