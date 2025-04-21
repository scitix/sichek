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

func getInfobyNvidiaSmi(ctx context.Context) *DeviceIndicatorStates {
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

	devIndicatorStates := &DeviceIndicatorStates{
		Indicators: make(map[string]*IndicatorStates),
	}

	// Convert each row of data into a map with headers as keys
	for _, row := range dataRows {
		gpuIndex := row[0]
		devIndicatorStates.Indicators[gpuIndex] = &IndicatorStates{
			Indicators: make(map[string]*IndicatorState),
			LastUpdate: time.Now(),
		}
		indicatorStates := devIndicatorStates.Indicators[gpuIndex].Indicators
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
			indicatorStates[header] = &IndicatorState{
				Active:   false,
				Value:    valueInt64,
				Duration: 0,
			}
		}
	}
	return devIndicatorStates
}

func (c *HangCollector) getInfobyLatestInfo(ctx context.Context) *DeviceIndicatorStates {
	var info *collector.NvidiaInfo
	var ok bool
	if !c.nvidiaComponent.Status() {
		_, err := common.RunHealthCheckWithTimeout(ctx, c.nvidiaComponent.GetTimeout(), c.nvidiaComponent.Name(), c.nvidiaComponent.HealthCheck)
		if err != nil {
			logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to get nvidia info according to health check")
			return nil
		}
	}
	infoRaw, err := c.nvidiaComponent.LastInfo()
	if err != nil {
		logrus.WithField("collector", "hanggetter").WithError(err).Errorf("failed to get last nvidia info")
		return nil
	}
	info, ok = infoRaw.(*collector.NvidiaInfo)
	if !ok {
		logrus.WithField("collector", "hanggetter").Errorf("wrong info type of last nvidia info")
		return nil
	}
	if !info.Time.After(c.LastUpdate) {
		logrus.WithField("collector", "hanggetter").Errorf("nvidia info not updated, current time: %v, last time: %v", info.Time, c.LastUpdate)
		return nil
	}

	devIndicatorStates := &DeviceIndicatorStates{
		Indicators: make(map[string]*IndicatorStates),
	}

	for i := range info.DeviceCount {
		deviceInfo := &info.DevicesInfo[i]
		uuid := deviceInfo.UUID
		// gpuIndexInt := deviceInfo.Index
		devIndicatorStates.Indicators[uuid] = &IndicatorStates{
			Indicators: make(map[string]*IndicatorState),
			LastUpdate: info.Time,
		}
		indicatorStates := devIndicatorStates.Indicators[uuid].Indicators
		for indicateName := range c.spec.Indicators {
			indicatorStates[indicateName] = &IndicatorState{
				Active:   false,
				Value:    0,
				Duration: 0,
			}
			// Get the value of the indicator
			var infoValue int64
			switch indicateName {
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
				infoValue = int64(deviceInfo.Clock.CurSMClk)
			case "gclk":
				infoValue = int64(deviceInfo.Clock.CurGraphicsClk)
			default:
				logrus.WithField("collector", "hanggetter").Errorf("failed to get info of %s", indicateName)
				continue
			}
			indicatorStates[indicateName].Value = infoValue
		}
	}
	return devIndicatorStates
}
