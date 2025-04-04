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
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/hang/checker"
	"github.com/scitix/sichek/components/hang/config"
	"github.com/scitix/sichek/components/nvidia"
	"github.com/scitix/sichek/components/nvidia/collector"

	"github.com/sirupsen/logrus"
)

type HangGetter struct {
	name string
	cfg  *config.HangUserConfig

	items         []string
	threshold     map[string]int64
	indicates     map[string]int64
	indicatesComp map[string]string
	prevTS        time.Time

	hangInfo checker.HangInfo

	nvidiaComponent common.Component
}

func NewHangGetter(ctx context.Context, cfg common.ComponentUserConfig) (hangGetter *HangGetter, err error) {
	hangCfg, ok := cfg.(*config.HangUserConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for Hang")
	}

	var res HangGetter
	res.name = hangCfg.Hang.Name
	res.cfg = hangCfg
	res.threshold = make(map[string]int64)
	res.indicates = make(map[string]int64)
	res.indicatesComp = make(map[string]string)

	if !hangCfg.Hang.Mock {
		res.nvidiaComponent = nvidia.GetComponent()
	} else {
		if res.nvidiaComponent, err = NewMockNvidiaComponent(""); err != nil {
			logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to NewNvidia")
			return nil, err
		}
	}

	for _, tmpCfg := range cfg.GetCheckerSpec() {
		getterConfig, ok := tmpCfg.(*config.HangErrorConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type for Hang getter")
		}
		threshold := getterConfig.HangThreshold
		for _, value := range getterConfig.HangIndicates {
			if value.Name != "pwr" && value.Name != "sm" &&
				value.Name != "gclk" && value.Name != "smclk" &&
				value.Name != "pviol" && value.Name != "rxpci" &&
				value.Name != "txpci" && value.Name != "mem" {
				logrus.WithField("collector", "hanggetter").
					Warnf("unsupport gpuhang indicate info type of %s", value.Name)
				continue
			}

			res.threshold[value.Name] = threshold
			res.indicates[value.Name] = value.Threshold
			res.indicatesComp[value.Name] = value.CompareFn
			res.items = append(res.items, value.Name)
		}
		res.prevTS = time.Time{}
	}

	res.hangInfo.Items = res.items
	res.hangInfo.HangThreshold = res.threshold
	res.hangInfo.HangDuration = make(map[string]map[string]int64)
	for j := 0; j < len(res.items); j++ {
		res.hangInfo.HangDuration[res.items[j]] = make(map[string]int64)
	}

	return &res, nil
}

func (c *HangGetter) Name() string {
	return c.name
}

func (c *HangGetter) GetCfg() common.ComponentUserConfig {
	return c.cfg
}

func (c *HangGetter) Collect(ctx context.Context) (common.Info, error) {
	c.hangInfo.Time = time.Now()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	getinfo, err := c.nvidiaComponent.CacheInfos(ctx)
	if err != nil {
		logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to get nvidia infos")
		return nil, err
	}

	if !c.nvidiaComponent.Status() {
		_, err := c.nvidiaComponent.HealthCheck(ctx)
		if err != nil {
			logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to run nvidiacomponent analyze")
		}
	}

	for k := 0; k < len(getinfo); k++ {
		if getinfo[k] == nil {
			continue
		}
		info, ok := getinfo[k].(*collector.NvidiaInfo)
		if !ok {
			logrus.WithField("collector", "hanggetter").WithError(err).Errorf("get wrong info type from nvidiaComponent")
			continue
		}
		if !info.Time.After(c.prevTS) {
			continue
		}

		for i := 0; i < info.DeviceCount; i++ {
			deviceInfo := &info.DevicesInfo[i]
			for j := 0; j < len(c.items); j++ {
				var infoValue int64
				switch c.items[j] {
				case "pwr":
					infoValue = int64(deviceInfo.Power.PowerUsage / 1000)
				case "mem":
					infoValue = int64(deviceInfo.Utilization.MemoryUsagePercent)
				case "sm":
					infoValue = int64(deviceInfo.Utilization.GPUUsagePercent)
				case "pviol":
					infoValue = int64(deviceInfo.Power.PowerViolations)
				case "rxpci":
					infoValue = int64(deviceInfo.PCIeInfo.PCIeRx / 1024)
				case "txpci":
					infoValue = int64(deviceInfo.PCIeInfo.PCIeTx / 1024)
				case "smclk":
					smClkInt := strings.Split(deviceInfo.Clock.CurSMClk, " ")[0]
					smClk, _ := strconv.Atoi(smClkInt)
					infoValue = int64(smClk)
				case "gclk":
					gClkInt := strings.Split(deviceInfo.Clock.CurGraphicsClk, " ")[0]
					gClk, _ := strconv.Atoi(gClkInt)
					infoValue = int64(gClk)
				default:
					logrus.WithField("collector", "hanggetter").Errorf("failed to get info of %s", c.items[j])
				}
				duration := c.getDuration(c.items[j], infoValue, info.Time)
				if duration == 0 {
					c.hangInfo.HangDuration[c.items[j]][deviceInfo.UUID] = 0
				} else {
					c.hangInfo.HangDuration[c.items[j]][deviceInfo.UUID] += duration
				}
				// logrus.WithField("collector", "hanggetter").Warnf("Index=%d, GPU=%d, item=%s, indicate=%d, threshold=%d, duration=%d, nowduration=%d\n",
				// 	deviceInfo.Index, i, c.items[j], infoValue, c.indicates[c.items[j]], duration,
				// 	c.hangInfo.HangDuration[c.items[j]][deviceInfo.UUID])
			}
		}
		c.prevTS = info.Time
	}

	return &c.hangInfo, nil
}

func (c *HangGetter) getDuration(name string, infoValue int64, now time.Time) int64 {
	var res int64 = 0
	if (infoValue < c.indicates[name] && c.indicatesComp[name] == "low") ||
		(infoValue > c.indicates[name] && c.indicatesComp[name] == "high") {
		if !c.prevTS.IsZero() {
			res = int64(now.Sub(c.prevTS).Seconds())
		}
	}
	return res
}
