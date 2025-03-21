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
	commonCfg "github.com/scitix/sichek/config"

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

// Checks if all CPUs are in "performance" mode
func (c *CPUPerfChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	cpu_performance_enable, err := checkCPUPerformance()
	if err != nil {
		return nil, fmt.Errorf("fail to check cpu performance: %v", err)
	}

	result := config.CPUCheckItems[CPUPerfCheckerName]

	if !cpu_performance_enable {
		err := setCPUMode("performance")
		if err == nil {
			cpu_performance_enable, err := checkCPUPerformance()
			if err != nil {
				return nil, fmt.Errorf("fail to check cpu performance2: %v", err)
			}
			if cpu_performance_enable {
				result.Status = commonCfg.StatusNormal
				result.Detail = "Not all CPUs are in \"performance\" mode. Already set all CPUs to \"performance\" mode successfully"
				result.Suggestion = ""
			} else {
				result.Status = commonCfg.StatusAbnormal
				result.Detail = "Not all CPUs are in \"performance\" mode. And failed to set all CPUs to \"performance\" mode"
			}
		} else {
			result.Status = commonCfg.StatusAbnormal
			result.Curr = "NotAllEnabled"
			result.Detail = fmt.Sprintf("Not all CPUs are in \"performance\" mode. And failed to set all CPUs to \"performance\" mode: %v", err)
		}
	} else {
		result.Status = commonCfg.StatusNormal
		result.Curr = "Enabled"
		result.Detail = "All CPUs are in \"performance\" mode"
		result.Suggestion = ""
	}
	return &result, nil
}

func checkCPUPerformance() (bool, error) {
	cpu_performance_enable := true
	// Path pattern to check CPU governor
	pattern := "/sys/devices/system/cpu/cpu*/cpufreq/scaling_governor"

	// Get all files matching the pattern
	files, err := filepath.Glob(pattern)
	if err != nil {
		cpu_performance_enable = false
		err = fmt.Errorf("failed to list CPU governor files: %w", err)
		return cpu_performance_enable, err
	}

	// If no governor files are found, return an error
	if len(files) == 0 {
		cpu_performance_enable = false
		err = fmt.Errorf("no CPU governor files found")
		return cpu_performance_enable, err
	}

	// Check each CPU's governor
	var err_cpus []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			err = fmt.Errorf("failed to read %s: %w", file, err)
			if err_cpus == nil {
				err_cpus = make([]string, 0, len(files))
			}
			cpu_performance_enable = false
			err_cpus = append(err_cpus, file)
			fmt.Println(err)
			continue
		}

		// Trim whitespace and check the mode
		cpu_mode := strings.TrimSpace(string(data))
		if cpu_mode != "performance" {
			if err_cpus == nil {
				err_cpus = make([]string, 0, len(files))
			}
			cpu_performance_enable = false
			err_cpus = append(err_cpus, file)
			// logrus.WithField("component", "Nvidia").Errorf("CPU %s is in %s mode", file, cpu_mode)
		}
	}
	// ret := fmt.Errorf("the following CPUs is not in performance mode: %v", err_cpus)
	return cpu_performance_enable, nil
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
			logrus.WithField("component", "Nvidia").Errorf("%v", ret)
			continue
		}
	}
	return ret
}
