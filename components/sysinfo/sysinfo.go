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
package sysinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/sysinfo/collector"
	"github.com/scitix/sichek/components/sysinfo/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
)

// SysinfoOutput is the component's snapshot payload: one SourceResult per source.
type SysinfoOutput struct {
	Sources map[string]*collector.SourceResult `json:"sources"`
}

func (o *SysinfoOutput) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string

	cfg      *config.SysinfoUserConfig
	cfgMutex sync.Mutex

	outputs    map[string]*collector.SourceResult
	outputsMtx sync.RWMutex

	resultCh chan *common.Result

	// source-goroutine lifecycle
	srcCancel context.CancelFunc
	srcWG     sync.WaitGroup

	runMtx  sync.Mutex
	running bool
}

var (
	sysinfoComponent *component
	sysinfoOnce      sync.Once
)

func NewComponent(cfgFile string, specFile string) (common.Component, error) {
	var err error
	sysinfoOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when create component sysinfo: %v", r)
			}
		}()
		sysinfoComponent, err = newComponent(cfgFile)
	})
	return sysinfoComponent, err
}

func newComponent(cfgFile string) (*component, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg, err := config.NewSysinfoUserConfig(cfgFile)
	if err != nil {
		cancel()
		return nil, err
	}
	return &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameSysinfo,
		cfg:           cfg,
		outputs:       make(map[string]*collector.SourceResult),
		resultCh:      make(chan *common.Result),
	}, nil
}

func (c *component) Name() string { return c.componentName }

func (c *component) GetTimeout() time.Duration {
	c.cfgMutex.Lock()
	defer c.cfgMutex.Unlock()
	return c.cfg.Sysinfo.QueryInterval.Duration
}

// HealthCheck runs every enabled source synchronously and returns a benign
// result. Used by the one-shot CLI path (RunComponentCheck).
func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	c.cfgMutex.Lock()
	sc := c.cfg.Sysinfo
	c.cfgMutex.Unlock()
	if sc.Enabled() {
		for _, src := range sc.Sources {
			if !sc.SourceEnabled(src) {
				continue
			}
			c.collectAndStore(ctx, src.Name, sc.ResolvedURL(src), sc.SourceTimeout(src))
		}
	}
	return c.benignResult(), nil
}

func (c *component) collectAndStore(ctx context.Context, name, url string, timeout time.Duration) {
	res := collector.Collect(ctx, name, url, timeout)
	c.outputsMtx.Lock()
	c.outputs[name] = res
	c.outputsMtx.Unlock()
	if res.Status != collector.StatusOK {
		logrus.WithField("component", "sysinfo").Warnf("source %q collect failed: %s", name, res.Error)
	}
}

func (c *component) benignResult() *common.Result {
	return &common.Result{
		Item:   c.componentName,
		Status: consts.StatusNormal,
		Level:  consts.LevelInfo,
		Time:   time.Now(),
	}
}

// Start spawns one goroutine per enabled source: run immediately, then loop on
// the source's own interval. Each run emits a benign result on the channel so
// the daemon picks up LastInfo() and updates the snapshot.
func (c *component) Start() <-chan *common.Result {
	c.runMtx.Lock()
	if c.running {
		c.runMtx.Unlock()
		return c.resultCh
	}
	c.running = true
	c.runMtx.Unlock()
	c.startSources()
	return c.resultCh
}

func (c *component) startSources() {
	c.cfgMutex.Lock()
	sc := c.cfg.Sysinfo
	c.cfgMutex.Unlock()

	sctx, scancel := context.WithCancel(c.ctx)
	c.srcCancel = scancel
	if !sc.Enabled() {
		return
	}
	for _, src := range sc.Sources {
		if !sc.SourceEnabled(src) {
			continue
		}
		c.srcWG.Add(1)
		go c.runSource(sctx, sc, src)
	}
}

func (c *component) runSource(ctx context.Context, sc *config.SysinfoConfig, src config.SourceSpec) {
	defer c.srcWG.Done()
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("component", "sysinfo").Errorf("panic in source %q: %v", src.Name, r)
		}
	}()
	url := sc.ResolvedURL(src)
	timeout := sc.SourceTimeout(src)
	ticker := time.NewTicker(sc.SourceInterval(src))
	defer ticker.Stop()

	// immediate first run
	c.collectAndStore(ctx, src.Name, url, timeout)
	if !c.send(ctx) {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectAndStore(ctx, src.Name, url, timeout)
			if !c.send(ctx) {
				return
			}
		}
	}
}

// send delivers a benign result unless the context is cancelled; the ctx guard
// prevents a send on a closed channel during Stop().
func (c *component) send(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case c.resultCh <- c.benignResult():
		return true
	}
}

func (c *component) Stop() error {
	if c.srcCancel != nil {
		c.srcCancel()
	}
	c.cancel()
	c.srcWG.Wait()
	c.runMtx.Lock()
	if c.running {
		close(c.resultCh)
		c.running = false
	}
	c.runMtx.Unlock()
	return nil
}

// Update swaps config and, if running, cancels + respawns the source goroutines
// so enable/disable, added/removed sources, and interval changes take effect
// without a restart.
func (c *component) Update(cfg common.ComponentUserConfig) error {
	newCfg, ok := cfg.(*config.SysinfoUserConfig)
	if !ok {
		return fmt.Errorf("update wrong config type for sysinfo")
	}
	if c.srcCancel != nil {
		c.srcCancel()
	}
	c.srcWG.Wait()
	c.cfgMutex.Lock()
	c.cfg = newCfg
	c.cfgMutex.Unlock()
	c.runMtx.Lock()
	running := c.running
	c.runMtx.Unlock()
	if running {
		c.startSources()
	}
	return nil
}

func (c *component) Status() bool {
	c.runMtx.Lock()
	defer c.runMtx.Unlock()
	return c.running
}

func (c *component) LastInfo() (common.Info, error) {
	c.outputsMtx.RLock()
	defer c.outputsMtx.RUnlock()
	cp := make(map[string]*collector.SourceResult, len(c.outputs))
	for k, v := range c.outputs {
		cp[k] = v
	}
	return &SysinfoOutput{Sources: cp}, nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	info, _ := c.LastInfo()
	return []common.Info{info}, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	return []*common.Result{c.benignResult()}, nil
}

func (c *component) LastResult() (*common.Result, error) {
	return c.benignResult(), nil
}

func (c *component) Metrics(ctx context.Context, since time.Time) (interface{}, error) {
	return nil, nil
}

// PrintInfo prints each source's status (and full KV when summaryPrint). It
// always returns true: this component makes no health verdict.
func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	out, ok := info.(*SysinfoOutput)
	if !ok {
		logrus.WithField("component", "sysinfo").Errorf("invalid data type, expected *SysinfoOutput")
		return false
	}
	names := make([]string, 0, len(out.Sources))
	for name := range out.Sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		src := out.Sources[name]
		fmt.Printf("sysinfo source %q: status=%s keys=%d source=%s\n", name, src.Status, src.KeyCount, src.Source)
		if src.Status != collector.StatusOK {
			fmt.Printf("  error: %s\n", src.Error)
		}
		if summaryPrint {
			keys := make([]string, 0, len(src.Raw))
			for k := range src.Raw {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("  %s=%s\n", k, src.Raw[k])
			}
		}
	}
	return true
}

// CollectOne loads config, runs a single named source once, and returns its
// result. Used by the CLI `--source` flag.
func CollectOne(cfgFile, name string) (*collector.SourceResult, error) {
	cfg, err := config.NewSysinfoUserConfig(cfgFile)
	if err != nil {
		return nil, err
	}
	sc := cfg.Sysinfo
	for _, src := range sc.Sources {
		if src.Name == name {
			return collector.Collect(context.Background(), name, sc.ResolvedURL(src), sc.SourceTimeout(src)), nil
		}
	}
	return nil, fmt.Errorf("no sysinfo source named %q", name)
}
