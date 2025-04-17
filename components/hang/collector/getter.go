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

	items              []string
	threshold          map[string]int64
	indicates          map[string]int64
	indicatesComp      map[string]string
	prevTS             time.Time
	prevIndicatorValue map[string]map[string]int64

	hangInfo checker.HangInfo

	nvidiaComponent common.Component
}

func NewHangGetter(hangCfg *config.HangUserConfig) (hangGetter *HangGetter, err error) {
	var res HangGetter
	res.name = hangCfg.Hang.Name
	res.cfg = hangCfg
	res.threshold = make(map[string]int64)
	res.indicates = make(map[string]int64)
	res.indicatesComp = make(map[string]string)
	res.prevIndicatorValue = make(map[string]map[string]int64)

	if !hangCfg.Hang.Mock {
		res.nvidiaComponent = nvidia.GetComponent()
	} else {
		if res.nvidiaComponent, err = NewMockNvidiaComponent(""); err != nil {
			logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to NewNvidia")
			return nil, err
		}
	}

	for _, getterConfig := range hangCfg.Hang.CheckerConfigs {
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
		res.prevIndicatorValue[res.items[j]] = make(map[string]int64)
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
	info, err := c.getLatestInfo(ctx)
	if err != nil || info == nil {
		logrus.WithField("collector", "hanggetter").Warnf("failed to get latest nvidia info")
		return nil, err
	}

	for i := range info.DeviceCount {
		deviceInfo := &info.DevicesInfo[i]
		uuid := deviceInfo.UUID

		for _, item := range c.items {
			var infoValue int64
			switch item {
			case "pwr":
				infoValue = int64(deviceInfo.Power.PowerUsage / 1000)
			case "mem":
				infoValue = int64(deviceInfo.Utilization.MemoryUsagePercent)
			case "sm":
				infoValue = int64(deviceInfo.Utilization.GPUUsagePercent)
			case "pviol":
				infoValue = int64(deviceInfo.Power.PowerViolations)
			case "rxpci":
				infoValueTmp := int64(deviceInfo.PCIeInfo.PCIeRx / 1024)
				if _, ok := c.prevIndicatorValue[item][uuid]; !ok {
					infoValue = infoValueTmp
				} else {
					infoValue = absDiff(infoValueTmp, c.prevIndicatorValue[item][uuid])
				}
				c.prevIndicatorValue[item][uuid] = infoValueTmp
			case "txpci":
				infoValueTmp := int64(deviceInfo.PCIeInfo.PCIeTx / 1024)
				if _, ok := c.prevIndicatorValue[item][uuid]; !ok {
					infoValue = infoValueTmp
				} else {
					infoValue = absDiff(infoValueTmp, c.prevIndicatorValue[item][uuid])
				}
				c.prevIndicatorValue[item][uuid] = infoValueTmp
			case "smclk":
				infoValue = int64(deviceInfo.Clock.CurSMClk)
			case "gclk":
				infoValue = int64(deviceInfo.Clock.CurGraphicsClk)
			default:
				logrus.WithField("collector", "hanggetter").Errorf("failed to get info of %s", item)
			}

			duration := c.getDuration(item, infoValue, info.Time)
			if duration == 0 {
				c.hangInfo.HangDuration[item][uuid] = 0
			} else {
				c.hangInfo.HangDuration[item][uuid] += duration
			}
		}
	}
	c.prevTS = info.Time

	return &c.hangInfo, nil
}

func (c *HangGetter) getLatestInfo(ctx context.Context) (*collector.NvidiaInfo, error) {
	var info *collector.NvidiaInfo
	var ok bool
	if !c.nvidiaComponent.Status() {
		_, err := common.RunHealthCheckWithTimeout(ctx, c.nvidiaComponent.GetTimeout(), c.nvidiaComponent.Name(), c.nvidiaComponent.HealthCheck)
		if err != nil {
			return nil, err
		}
	}
	infoRaw, err := c.nvidiaComponent.LastInfo()
	if err != nil {
		return nil, err
	}
	info, ok = infoRaw.(*collector.NvidiaInfo)
	if !ok {
		return nil, fmt.Errorf("hanggetter: wrong info type of last nvidia info")
	}
	if !info.Time.After(c.prevTS) {
		return nil, fmt.Errorf("hanggetter: nvidia info not updated, current time: %v, last time: %v", info.Time, c.prevTS)
	}

	return info, nil
}

func absDiff(a, b int64) int64 {
	if a > b {
		return a - b
	}
	return b - a
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
