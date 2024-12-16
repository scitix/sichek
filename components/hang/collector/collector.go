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
	HangCfg "github.com/scitix/sichek/components/hang/config"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type HangCollector struct {
	name string
	cfg  *HangCfg.HangConfig

	items         []string
	threshold     map[string]int64
	indicates     map[string]int64
	indicatesComp map[string]string
	prevTS        time.Time
	hangInfo      checker.HangInfo
}

func NewHangCollector(ctx context.Context, cfg common.ComponentConfig) (*HangCollector, error) {
	config, ok := cfg.(*HangCfg.HangConfig)
	if !ok {
		return nil, fmt.Errorf("invalid config type for Hang")
	}

	var res HangCollector
	res.name = config.Name
	res.cfg = config
	res.threshold = make(map[string]int64)
	res.indicates = make(map[string]int64)
	res.indicatesComp = make(map[string]string)

	for _, tmpCfg := range cfg.GetCheckerSpec() {
		collectorConfig, ok := tmpCfg.(*HangCfg.HangErrorConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type for Hang collector")
		}
		threshold := collectorConfig.HangThreshold
		for _, value := range collectorConfig.HangIndicates {
			// value.Name != "gclk" && value.Name != "smclk" &&
			if value.Name != "pwr" && value.Name != "sm" &&
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
	}

	res.hangInfo.Items = res.items
	res.hangInfo.HangThreshold = res.threshold
	res.hangInfo.HangDuration = make(map[string]map[string]int64)
	for j := 0; j < len(res.items); j++ {
		res.hangInfo.HangDuration[res.items[j]] = make(map[string]int64)
	}

	return &res, nil
}

func (c *HangCollector) Name() string {
	return c.name
}

func (c *HangCollector) GetCfg() common.ComponentConfig {
	return c.cfg
}

func (c *HangCollector) Collect() (common.Info, error) {
	c.hangInfo.Time = time.Now()

	gpusInfo := getGPUInfo()
	now := time.Now()

	for i := 0; i < len(c.items); i++ {
		for j := 0; j < len(gpusInfo); j++ {
			gpuInfo := gpusInfo[j]
			indicateName := c.items[i]

			v, err := strconv.ParseInt(gpuInfo[indicateName], 10, 64)
			if err != nil {
				logrus.WithField("collector", "Hang").WithError(err).Errorf("failed to parse gpu res to int64, %s->%s", indicateName, gpuInfo[indicateName])
				continue
			}
			if ((v < c.indicates[indicateName]) && (c.indicatesComp[indicateName] == "high")) ||
				((v > c.indicates[indicateName]) && (c.indicatesComp[indicateName] == "low")) {
				c.hangInfo.HangDuration[indicateName][gpuInfo["gpu"]] = 0
			} else {
				c.hangInfo.HangDuration[indicateName][gpuInfo["gpu"]] += int64(now.Sub(c.prevTS).Seconds())
			}
		}
	}
	c.prevTS = now
	return &c.hangInfo, nil
}

func getGPUInfo() []map[string]string {
	// cmd := exec.Command("nvidia-smi", "dmon", "-s", "pucvmet", "-d", "10", "-c", "1")
	// var out bytes.Buffer
	// cmd.Stdout = &out
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
		logrus.WithField("collector", "Hang").WithError(err).Errorf("No valid data found in nvidia-smi output")
		return nil
	}

	results := make([]map[string]string, 0, len(dataRows))
	for _, row := range dataRows {
		rowMap := make(map[string]string)
		for i, header := range headers {
			if i < len(row) {
				rowMap[header] = row[i]
			}
		}
		results = append(results, rowMap)
	}
	// fmt.Println("Parsed GPU Data:")
	// for i, result := range results {
	// 	fmt.Printf("Row %d:\n", i)
	// 	for k, v := range result {
	// 		fmt.Printf("  %s: %s\n", k, v)
	// 	}
	// }
	return results
}
