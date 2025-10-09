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
package syslog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/scitix/sichek/pkg/utils"

	"github.com/scitix/sichek/components/common"
	filter "github.com/scitix/sichek/components/common/eventfilter"
	"github.com/scitix/sichek/components/syslog/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

type component struct {
	componentName string
	ctx           context.Context
	cancel        context.CancelFunc

	cfg      *config.SyslogUserConfig
	cfgMutex sync.RWMutex
	filter   *filter.EventFilter

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	healthCheckMtx sync.Mutex

	service *common.CommonService
}

var (
	syslogComponent     *component
	syslogComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string, skipPercent int64) (common.Component, error) {
	var err error
	syslogComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component syslog: %v", r)
			}
		}()
		syslogComponent, err = newSyslogComponent(cfgFile, specFile, skipPercent)
	})
	return syslogComponent, err
}

func newSyslogComponent(cfgFile string, eventRulesFile string, skipPercent int64) (comp *component, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.SyslogUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Syslog == nil {
		logrus.WithField("component", "syslog").Errorf("NewComponent get config failed or user config is nil, err: %v", err)
		return nil, fmt.Errorf("NewSyslogComponent get user config failed")
	}

	eventRules, err := config.LoadEventRules(eventRulesFile)
	if err != nil {
		logrus.WithField("component", "syslog").Errorf("failed to NewComponent: %v", err)
		return nil, err
	}

	// if skipPercent is -1, use the value from the config file
	if skipPercent == -1 {
		skipPercent = cfg.Syslog.SkipPercent
	}
	filterPointer, err := filter.NewEventFilter(consts.ComponentNameSyslog, eventRules, skipPercent)
	if err != nil {
		logrus.WithField("component", "syslog").Errorf("failed to create Syslog EventFilter: %v", err)
		return nil, err
	}

	component := &component{
		componentName: consts.ComponentNameSyslog,
		ctx:           ctx,
		cancel:        cancel,
		cfg:           cfg,
		cfgMutex:      sync.RWMutex{},
		filter:        filterPointer,
		cacheMtx:      sync.RWMutex{},
		cacheBuffer:   make([]*common.Result, cfg.Syslog.CacheSize),
		cacheInfo:     make([]common.Info, cfg.Syslog.CacheSize),
		currIndex:     0,
		cacheSize:     cfg.Syslog.CacheSize,
	}

	component.service = common.NewCommonService(ctx, cfg, component.componentName, component.GetTimeout(), component.HealthCheck)

	return component, nil
}

func (c *component) Name() string {
	return c.componentName
}

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	c.healthCheckMtx.Lock()
	defer c.healthCheckMtx.Unlock()

	timer := common.NewTimer(fmt.Sprintf("%s-HealthCheck-Cost", c.componentName))

	// Run event filter check
	eventResult := c.filter.Check()
	eventResult.Item = c.componentName
	timer.Mark("event-filter")

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = eventResult
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	if eventResult.Status == consts.StatusAbnormal {
		logrus.WithField("component", "syslog").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "syslog").Infof("Health Check PASSED")
	}

	return eventResult, nil
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
	return nil, fmt.Errorf("no info for syslog")
}

func (c *component) LastInfo() (common.Info, error) {
	return nil, fmt.Errorf("no info for syslog")
}

func (c *component) Start() <-chan *common.Result {
	return c.service.Start()
}

func (c *component) Stop() error {
	return c.service.Stop()
}

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	configPointer, ok := cfg.(*config.SyslogUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for syslog")
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

func (c *component) PrintInfo(info common.Info /*ignored*/, result *common.Result, summaryPrint bool) bool {
	checkAllPassed := true
	var syslogEvents = make(map[string]string)

	utils.PrintTitle("System Log Events", "-")
	if result != nil {
		checkerResults := result.Checkers
		for _, result := range checkerResults {
			if result.Status == consts.StatusAbnormal {
				checkAllPassed = false
				syslogEvents[result.Name] = fmt.Sprintf("%sDetect %s %v times in %s %s", consts.Red, result.ErrorName, result.Curr, result.Device, consts.Reset)
			}
		}
	}
	if checkAllPassed {
		fmt.Printf("%sNo System Log Error detected%s\n", consts.Green, consts.Reset)
	} else {
		for _, v := range syslogEvents {
			fmt.Printf("\t%s\n", v)
		}
	}
	return checkAllPassed
}
