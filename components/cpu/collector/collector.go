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
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/pkg/utils/filter"

	"github.com/sirupsen/logrus"
)

type CPUOutput struct {
	Time         time.Time                         `json:"time"`
	CPUArchInfo  CPUArchInfo                       `json:"cpu_arch_info"`
	UsageInfo    Usage                             `json:"cpu_usage_info"`
	HostInfo     HostInfo                          `json:"host_info"`
	Uptime       string                            `json:"uptime"`
	EventResults map[string][]*filter.FilterResult `json:"event_results"`
}

func (o *CPUOutput) JSON() (string, error) {
	data, err := json.Marshal(o)
	return string(data), err
}

type collector struct {
	name        string
	cfg         *config.CPUConfig
	CPUArchInfo *CPUArchInfo `json:"cpu_arch_info"`
	HostInfo    *HostInfo    `json:"host_info"`
	filter      *filter.FileFilter
}

func NewCpuCollector(ctx context.Context, cfg common.ComponentConfig) (*collector, error) {
	config, ok := cfg.(*config.CPUConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for CPU")
	}
	filterNames := make([]string, 0)
	regexps := make([]string, 0)
	files_map := make(map[string]bool)
	files := make([]string, 0)
	for _, checker_cfg := range config.EventCheckers {
		_, err := os.Stat(checker_cfg.LogFile)
		if err != nil {
			logrus.WithField("collector", "CPU").Errorf("log file %s not exist for CPU collector", checker_cfg.LogFile)
			continue
		}
		filterNames = append(filterNames, checker_cfg.Name)
		if _, exist := files_map[checker_cfg.LogFile]; !exist {
			files = append(files, checker_cfg.LogFile)
			files_map[checker_cfg.LogFile] = true
		}
		regexps = append(regexps, checker_cfg.Regexp)
	}

	filter, err := filter.NewFileFilter(filterNames, regexps, files, 1)
	if err != nil {
		return nil, err
	}

	collector := &collector{
		name:   "CPUCollector",
		cfg:    config,
		filter: filter,
	}
	collector.CPUArchInfo = &CPUArchInfo{}
	if err := collector.CPUArchInfo.Get(ctx); err != nil {
		return nil, err
	}
	collector.HostInfo = &HostInfo{}
	if err := collector.HostInfo.Get(); err != nil {
		return nil, err
	}
	return collector, nil
}

func (c *collector) Name() string {
	return c.name
}

func (c *collector) GetCfg() common.ComponentConfig {
	return c.cfg
}

func (c *collector) Collect() (common.Info, error) {
	cpuOutput := &CPUOutput{
		Time:        time.Now(),
		CPUArchInfo: *c.CPUArchInfo,
		HostInfo:    *c.HostInfo,
	}
	err := cpuOutput.UsageInfo.Get()
	if err != nil {
		return nil, fmt.Errorf("get CPU Usage info failed: %v", err)
	}

	cpuOutput.Uptime, err = GetUptime()
	if err != nil {
		return nil, fmt.Errorf("get uptime failed: %v", err)
	}

	filterRes := c.filter.Check()
	filterResMap := make(map[string][]*filter.FilterResult)
	for _, res := range filterRes {
		filterResMap[res.Name] = append(filterResMap[res.Name], &res)
	}
	cpuOutput.EventResults = filterResMap

	return cpuOutput, nil
}

// GetUptime returns the system uptime as a formatted string.
func GetUptime() (string, error) {
	// Get uptime (Linux-specific example)
	uptimeBytes, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "", fmt.Errorf("failed to ReadFile /proc/uptime: %w", err)
	}
	uptimeFields := strings.Fields(string(uptimeBytes))
	uptimeSeconds, err := strconv.ParseFloat(uptimeFields[0], 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse uptime: %w", err)
	}
	uptimeDuration := time.Duration(uptimeSeconds) * time.Second
	days := uptimeDuration / (24 * time.Hour)
	uptimeDuration -= days * 24 * time.Hour
	hours := uptimeDuration / time.Hour
	uptimeDuration -= hours * time.Hour
	minutes := uptimeDuration / time.Minute
	uptimeDuration -= minutes * time.Minute
	seconds := uptimeDuration / time.Second
	return fmt.Sprintf("%dd, %02d:%02d:%02d", days, hours, minutes, seconds), nil
}
