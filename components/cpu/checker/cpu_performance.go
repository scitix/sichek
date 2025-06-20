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
package checker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/cpu/config"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

const CPUPerfCheckerName = "cpu-performance"

type CPUPerfChecker struct {
	name string
}

func NewCPUPerfChecker() (common.Checker, error) {
	return &CPUPerfChecker{
		name: CPUPerfCheckerName,
	}, nil
}

func (c *CPUPerfChecker) Name() string {
	return c.name
}

func (c *CPUPerfChecker) GetSpec() common.CheckerSpec {
	return nil
}

// Check Checks if all CPUs are in "performance" mode
func (c *CPUPerfChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpuPerformanceEnable, err := checkCPUPerformance()
	if err != nil {
		return nil, fmt.Errorf("fail to check cpu performance: %v", err)
	}

	result := config.CPUCheckItems[CPUPerfCheckerName]

	if !cpuPerformanceEnable {
		err := setCPUMode("performance")
		if err == nil {
			cpuPerformanceEnable, err := checkCPUPerformance()
			if err != nil {
				return nil, fmt.Errorf("fail to check cpu performance2: %v", err)
			}
			if cpuPerformanceEnable {
				result.Status = consts.StatusNormal
				result.Detail = "Not all CPUs are in \"performance\" mode. Already set all CPUs to \"performance\" mode successfully"
				result.Suggestion = ""
			} else {
				result.Status = consts.StatusAbnormal
				result.Detail = "Not all CPUs are in \"performance\" mode. And failed to set all CPUs to \"performance\" mode"
			}
		} else {
			result.Status = consts.StatusAbnormal
			result.Curr = "NotAllEnabled"
			result.Detail = fmt.Sprintf("Not all CPUs are in \"performance\" mode. And failed to set all CPUs to \"performance\" mode: %v", err)
		}
	} else {
		result.Status = consts.StatusNormal
		result.Curr = "Enabled"
		result.Detail = "All CPUs are in \"performance\" mode"
		result.Suggestion = ""
	}
	return &result, nil
}

func checkCPUPerformance() (bool, error) {
	cpuPerformanceEnable := true
	// Path pattern to check CPU governor
	pattern := "/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor"

	// Get all files matching the pattern
	files, err := filepath.Glob(pattern)
	if err != nil {
		cpuPerformanceEnable = false
		err = fmt.Errorf("failed to list CPU governor files: %w", err)
		return cpuPerformanceEnable, err
	}

	// If no governor files are found, return an error
	if len(files) == 0 {
		cpuPerformanceEnable = false
		err = fmt.Errorf("no CPU governor files found")
		return cpuPerformanceEnable, err
	}

	// Check each CPU's governor
	var errCpus []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			err = fmt.Errorf("failed to read %s: %w", file, err)
			if errCpus == nil {
				errCpus = make([]string, 0, len(files))
			}
			cpuPerformanceEnable = false
			errCpus = append(errCpus, file)
			fmt.Println(err)
			continue
		}

		// Trim whitespace and check the mode
		cpuMode := strings.TrimSpace(string(data))
		if cpuMode != "performance" {
			if errCpus == nil {
				errCpus = make([]string, 0, len(files))
			}
			cpuPerformanceEnable = false
			errCpus = append(errCpus, file)
		}
	}
	// ret := fmt.Errorf("the following CPUs is not in performance mode: %v", err_cpus)
	return cpuPerformanceEnable, nil
}

func setCPUMode(mode string) error {
	// Path pattern to check CPU governor
	pattern := "/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor"

	// Get all files matching the pattern
	files, err := filepath.Glob(pattern)
	if err != nil {
		err = fmt.Errorf("failed to list CPU governor files: %w", err)
		return err
	}

	// If no governor files are found, return an error
	if len(files) == 0 {
		err = fmt.Errorf("no CPU governor files found")
		return err
	}

	// Check each CPU's governor
	var ret error
	for _, file := range files {
		f, err := os.OpenFile(file, os.O_WRONLY, 0644)
		if err != nil {
			ret = fmt.Errorf("failed to open %s: %v", file, err)
			continue
		}

		// Write mode(e.g. "performance") to the governor file
		_, err = f.WriteString(mode)
		if err != nil {
			ret = fmt.Errorf("failed to write %s to %s: %v", mode, file, err)
			logrus.WithField("component", "cpu").Errorf("%v", ret)
			continue
		}
	}
	return ret
}
