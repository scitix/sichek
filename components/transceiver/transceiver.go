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
package transceiver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/checker"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	trmetrics "github.com/scitix/sichek/components/transceiver/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.TransceiverUserConfig
	cfgMutex      sync.Mutex
	collector     *collector.TransceiverCollector
	checkers      []common.Checker
	metrics       *trmetrics.TransceiverMetrics

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	transceiverComponent     *component
	transceiverComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string, ignoredCheckers []string) (common.Component, error) {
	var err error
	transceiverComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component transceiver: %v", r)
			}
		}()
		transceiverComponent, err = newComponent(cfgFile, specFile, ignoredCheckers)
	})
	return transceiverComponent, err
}

func newComponent(cfgFile string, specFile string, ignoredCheckers []string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.TransceiverUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Transceiver == nil {
		logrus.WithField("component", "transceiver").Warnf("get user config failed or transceiver config is nil, using default config")
		cfg.Transceiver = &config.TransceiverConfig{
			QueryInterval: common.Duration{Duration: 60 * time.Second},
			CacheSize:     5,
		}
	}
	if len(ignoredCheckers) > 0 {
		cfg.Transceiver.IgnoredCheckers = ignoredCheckers
	}

	spec, err := config.LoadSpec(specFile)
	if err != nil {
		logrus.WithField("component", "transceiver").Warnf("failed to load spec %s: %v", specFile, err)
	}

	// Build classifier from spec: speed-based + pattern-based
	patterns := make(map[string][]string)
	managementMaxMbps := 0
	if spec != nil {
		for netName, netSpec := range spec.Networks {
			patterns[netName] = netSpec.InterfacePatterns
			if netName == "management" && netSpec.MaxSpeedMbps > 0 {
				managementMaxMbps = netSpec.MaxSpeedMbps
			}
		}
	}
	classifier := collector.NewNetworkClassifier(patterns, managementMaxMbps)

	collectorInst := collector.NewTransceiverCollector(classifier)

	checkers, err := checker.NewCheckers(cfg, spec)
	if err != nil {
		return nil, err
	}

	cacheSize := cfg.Transceiver.CacheSize
	if cacheSize == 0 {
		cacheSize = 5
	}

	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameTransceiver,
		collector:     collectorInst,
		checkers:      checkers,
		cfg:           cfg,
		cacheBuffer:   make([]*common.Result, cacheSize),
		cacheInfo:     make([]common.Info, cacheSize),
		cacheSize:     cacheSize,
		metrics:       trmetrics.NewTransceiverMetrics(),
	}
	service := common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	comp.service = service

	return comp, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	timer := common.NewTimer(fmt.Sprintf("%s-HealthCheck-Cost", c.componentName))
	trInfo, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "transceiver").Errorf("failed to collect transceiver info: %v", err)
		return nil, err
	}
	logrus.WithField("component", "transceiver").Infof("collected transceiver info: %d modules", len(trInfo.Modules))

	if c.cfg.Transceiver != nil && c.cfg.Transceiver.EnableMetrics {
		c.metrics.ExportMetrics(trInfo)
	}

	result := common.Check(ctx, c.componentName, trInfo, c.checkers)
	timer.Mark("transceiver-check")

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = trInfo
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	if result.Status == consts.StatusAbnormal && consts.LevelPriority[result.Level] > consts.LevelPriority[consts.LevelInfo] {
		logrus.WithField("component", "transceiver").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "transceiver").Infof("Health Check PASSED")
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
	configPointer, ok := cfg.(*config.TransceiverUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for transceiver")
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
	if result.Status == consts.StatusAbnormal && consts.LevelPriority[result.Level] > consts.LevelPriority[consts.LevelInfo] {
		checkAllPassed = false
	}

	utils.PrintTitle("Transceiver", "-")

	trInfo, ok := info.(*collector.TransceiverInfo)
	if !ok || trInfo == nil {
		fmt.Println("No transceiver info available")
		return checkAllPassed
	}

	if len(trInfo.Modules) == 0 {
		fmt.Println("No transceiver modules found")
		return checkAllPassed
	}

	fmt.Printf("%-16s %-12s %-12s %-20s %-12s %-8s %-10s %-24s %-24s\n",
		"Interface", "NetworkType", "Vendor", "PartNumber", "LinkSpeed", "Temp(C)", "Volt(V)", "TxPower(dBm)", "RxPower(dBm)")
	fmt.Printf("%-16s %-12s %-12s %-20s %-12s %-8s %-10s %-24s %-24s\n",
		"----------------", "------------", "------------", "--------------------", "------------", "--------", "----------", "------------------------", "------------------------")
	for _, mod := range trInfo.Modules {
		speed := mod.LinkSpeed
		if speed == "" {
			speed = "-"
		}
		txStr := formatLanePower(mod.TxPower)
		rxStr := formatLanePower(mod.RxPower)
		voltStr := "-"
		if mod.Voltage > 0 {
			voltStr = fmt.Sprintf("%.3f", mod.Voltage)
		}
		fmt.Printf("%-16s %-12s %-12s %-20s %-12s %-8.1f %-10s %-24s %-24s\n",
			mod.Interface, mod.NetworkType, mod.Vendor, mod.PartNumber, speed, mod.Temperature, voltStr, txStr, rxStr)
	}

	if result != nil && len(result.Checkers) > 0 {
		hasErrors := false
		for _, res := range result.Checkers {
			if res.Status != consts.StatusNormal && res.Level != consts.LevelInfo {
				if !hasErrors {
					fmt.Printf("\nErrors Events:\n")
					hasErrors = true
				}
				fmt.Printf("\tEvent: %s%s%s -> %s\n", consts.Red, res.ErrorName, consts.Reset, res.Detail)
			}
		}
		if !hasErrors {
			fmt.Printf("\nErrors Events:\n\tNo Transceiver Events Detected\n")
		}
	} else {
		fmt.Printf("\nErrors Events:\n\tNo Transceiver Events Detected\n")
	}

	fmt.Println()
	return checkAllPassed
}

func formatLanePower(powers []float64) string {
	if len(powers) == 0 {
		return "-"
	}
	parts := make([]string, len(powers))
	for i, p := range powers {
		parts[i] = fmt.Sprintf("%.2f", p)
	}
	return strings.Join(parts, ",")
}
