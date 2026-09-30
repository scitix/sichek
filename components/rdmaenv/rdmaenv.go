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

// Package rdmaenv is a passthrough component: it scrapes the co-located rdma-env-pre
// exporter (:19099/metrics), re-exports its series verbatim on sichek's /metrics, and
// records a per-knob desired/observed digest in the snapshot. It does no health
// judgment: HealthCheck runs no checkers and always reports normal.
package rdmaenv

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/rdmaenv/collector"
	"github.com/scitix/sichek/components/rdmaenv/config"
	rmetrics "github.com/scitix/sichek/components/rdmaenv/metrics"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.RdmaEnvUserConfig
	cfgMutex      sync.Mutex
	collector     *collector.Collector
	metrics       *rmetrics.RdmaEnvMetrics

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	rdmaEnvComponent     *component
	rdmaEnvComponentOnce sync.Once
)

// NewComponent builds (once) the rdmaenv component. specFile is accepted for factory
// signature symmetry but unused: this component has no hardware spec baseline.
func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	rdmaEnvComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component rdmaenv: %v", r)
			}
		}()
		rdmaEnvComponent, err = newComponent(cfgFile)
	})
	return rdmaEnvComponent, err
}

func newComponent(cfgFile string) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.RdmaEnvUserConfig{}
	if e := common.LoadUserConfig(cfgFile, cfg); e != nil || cfg.RdmaEnv == nil {
		logrus.WithField("component", "rdmaenv").Warnf("get user config failed or rdmaenv config is nil, using default config")
		cfg.RdmaEnv = &config.RdmaEnvConfig{
			QueryInterval: common.Duration{Duration: 10 * time.Minute},
			Endpoint:      "http://127.0.0.1:19099/metrics",
			Timeout:       common.Duration{Duration: 10 * time.Second},
			MetricPrefix:  "rdma_env_pre_",
			EnableMetrics: true,
		}
	}

	collectorInst := collector.NewCollector(cfg.RdmaEnv.Endpoint, cfg.RdmaEnv.Timeout.Duration, cfg.RdmaEnv.MetricPrefix)

	const cacheSize = 5
	comp = &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameRdmaEnv,
		cfg:           cfg,
		collector:     collectorInst,
		metrics:       rmetrics.NewRdmaEnvMetrics(),
		cacheBuffer:   make([]*common.Result, cacheSize),
		cacheInfo:     make([]common.Info, cacheSize),
		cacheSize:     cacheSize,
	}
	comp.service = common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	return comp, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "rdmaenv").Errorf("failed to collect rdmaenv info: %v", err)
		return nil, err
	}
	if c.cfg.RdmaEnv != nil && c.cfg.RdmaEnv.EnableMetrics {
		c.metrics.ExportMetrics(info)
	}

	// No checkers: passthrough never judges. Check with a nil checker set yields a
	// normal, info-level result.
	result := common.Check(ctx, c.componentName, info, nil)

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = info
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	if info.Available {
		logrus.WithField("component", "rdmaenv").Infof("rdma-env-pre scrape OK: %d series, host=%s", info.Summary.SeriesTotal, info.Summary.HostCompliance)
	} else {
		logrus.WithField("component", "rdmaenv").Warnf("rdma-env-pre unavailable at %s: %s", info.Endpoint, info.Error)
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

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.RdmaEnvUserConfig)
	if !ok {
		c.cfgMutex.Unlock()
		return fmt.Errorf("update wrong config type for rdmaenv")
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

// PrintInfo prints the passthrough digest. It always returns true: passthrough does not
// participate in pass/fail.
func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	utils.PrintTitle("RdmaEnv", "-")

	rInfo, ok := info.(*collector.Info)
	if !ok || rInfo == nil {
		fmt.Println("No rdmaenv info available")
		return true
	}

	fmt.Printf("Endpoint: %s\n", rInfo.Endpoint)
	if !rInfo.Available {
		fmt.Printf("Available: false  (%s)\n", rInfo.Error)
		return true
	}

	fmt.Printf("Available: true   Series: %d   HostCompliance: %s\n",
		rInfo.Summary.SeriesTotal, rInfo.Summary.HostCompliance)
	if len(rInfo.Summary.VerdictCounts) > 0 {
		fmt.Printf("VerdictCounts: %v\n", rInfo.Summary.VerdictCounts)
	}

	printed := false
	for _, k := range rInfo.Summary.Knobs {
		if k.Verdict == "converged" {
			continue
		}
		if !printed {
			fmt.Printf("\nNon-converged knobs:\n")
			fmt.Printf("%-18s %-26s %-10s %-14s %-14s\n", "Device", "Knob", "Verdict", "Desired", "Observed")
			printed = true
		}
		fmt.Printf("%-18s %-26s %-10s %-14s %-14s\n", k.Device, k.Knob, k.Verdict, k.Desired, k.Observed)
	}
	if !printed {
		fmt.Printf("\nAll knobs converged\n")
	}
	fmt.Println()
	return true
}
