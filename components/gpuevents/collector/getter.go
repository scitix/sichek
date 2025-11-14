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
	"strconv"
	"strings"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/nvidia/collector"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

func getInfobyNvidiaSmi(ctx context.Context) *DeviceIndicatorValues {
	out, err := utils.ExecCommand(ctx, "nvidia-smi", "dmon", "-s", "pucvmet", "-d", "10", "-c", "1")
	if err != nil {
		logrus.WithField("collector", "Hang").WithError(err).Errorf("Error running command:")
		return nil
	}
	output := string(out)
	lines := strings.Split(output, "\n")

	var headers []string
	var dataRows [][]string

	for _, line := range lines {
		if strings.HasPrefix(line, "#") && strings.Contains(line, "gpu") {
			headers = strings.Fields(line[1:])
		} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") {
			dataRows = append(dataRows, strings.Fields(line))
		}
	}

	if len(headers) == 0 || len(dataRows) == 0 {
		logrus.WithField("collector", "Hang").Errorf("No valid data found in nvidia-smi output")
		return nil
	}

	devIndicatorValues := &DeviceIndicatorValues{
		Indicators: make(map[string]*IndicatorValues),
	}

	// Convert each row of data into a map with headers as keys
	for _, row := range dataRows {
		gpuIndex := row[0]
		devIndicatorValues.Indicators[gpuIndex] = &IndicatorValues{
			Indicators: make(map[string]int64),
			LastUpdate: time.Now(),
		}
		IndicatorValues := devIndicatorValues.Indicators[gpuIndex].Indicators
		for i, header := range headers {
			value := row[i]
			valueInt64, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				logrus.WithField("collector", "Hang").Errorf("failed to parse gpu res to int64, %s->%s", header, value)
				continue
			}
			if i == 0 {
				continue // Skip the first header (usually "gpu")
			}
			IndicatorValues[header] = valueInt64
		}
	}
	devIndicatorValues.LastUpdate = time.Now()
	return devIndicatorValues
}

func (c *GpuIndicatorSnapshot) getInfobyLatestInfo(ctx context.Context) *DeviceIndicatorValues {
	var info *collector.NvidiaInfo
	var ok bool
	if !c.nvidiaComponent.Status() {
		_, err := common.RunHealthCheckWithTimeout(ctx, c.nvidiaComponent.GetTimeout(), c.nvidiaComponent.Name(), c.nvidiaComponent.HealthCheck)
		if err != nil {
			logrus.WithField("collector", "gpuevents").WithError(err).Errorf("failed to get nvidia info according to health check")
			return nil
		}
	}
	infoRaw, err := c.nvidiaComponent.LastInfo()
	if err != nil {
		logrus.WithField("collector", "gpuevents").WithError(err).Errorf("failed to get last nvidia info")
		return nil
	}
	info, ok = infoRaw.(*collector.NvidiaInfo)
	if !ok {
		logrus.WithField("collector", "gpuevents").Errorf("wrong info type of last nvidia info")
		return nil
	}
	if !info.Time.After(c.LastUpdate) {
		logrus.WithField("collector", "gpuevents").Warnf("nvidia info not updated, current time: %s, last time: %s", info.Time, c.LastUpdate)
		return nil
	}

	logrus.WithField("collector", "gpuevents").Infof("get updated nvidia info, current time: %s, last time: %s", info.Time, c.LastUpdate)

	devIndicatorValues := &DeviceIndicatorValues{
		Indicators: make(map[string]*IndicatorValues),
	}

	for i := range info.DevicesInfo {
		deviceInfo := &info.DevicesInfo[i]
		uuid := deviceInfo.UUID
		// gpuIndexInt := deviceInfo.Index
		devIndicatorValues.Indicators[uuid] = &IndicatorValues{
			Indicators: make(map[string]int64),
			LastUpdate: info.Time,
		}
		IndicatorValues := devIndicatorValues.Indicators[uuid].Indicators
		// Get the value of the indicator
		IndicatorValues["pwr"] = int64(deviceInfo.Power.PowerUsage / 1000)
		IndicatorValues["mem"] = int64(deviceInfo.Utilization.MemoryUsagePercent)
		IndicatorValues["sm"] = int64(deviceInfo.Utilization.GPUUsagePercent)
		IndicatorValues["smclk"] = int64(deviceInfo.Clock.CurSMClk)
		IndicatorValues["gclk"] = int64(deviceInfo.Clock.CurGraphicsClk)
		IndicatorValues["pviol"] = int64(deviceInfo.Power.PowerViolations)
		IndicatorValues["rxpci"] = int64(deviceInfo.PCIeInfo.PCIeRx / 1024)
		IndicatorValues["txpci"] = int64(deviceInfo.PCIeInfo.PCIeTx / 1024)
		if !deviceInfo.ClockEvents.IsSupported {
			IndicatorValues["gpuidle"] = -1
		} else if deviceInfo.ClockEvents.GpuIdle {
			IndicatorValues["gpuidle"] = 1
		} else {
			IndicatorValues["gpuidle"] = 0
		}
	}
	// Update the last update time
	devIndicatorValues.LastUpdate = info.Time
	return devIndicatorValues
}
