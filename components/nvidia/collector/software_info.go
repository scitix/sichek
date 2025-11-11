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
	"regexp"
	"strconv"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/pkg/utils"
)

type SoftwareInfo struct {
	DriverVersion string `json:"driver_version" yaml:"driver_version"`
	CUDAVersion   string `json:"cuda_version,omitempty" yaml:"cuda_version,omitempty"`
}

func (s *SoftwareInfo) JSON() ([]byte, error) {
	return common.JSON(s)
}

// ToString Convert struct to JSON (pretty-printed)
func (s *SoftwareInfo) ToString() string {
	return common.ToString(s)
}

func (s *SoftwareInfo) Get(ctx context.Context, devIndex int) error {
	// Run the nvidia-smi command
	out, err := utils.ExecCommand(ctx, "nvidia-smi", "-q", "-i", strconv.Itoa(devIndex))
	if err != nil {
		return fmt.Errorf("failed to run nvidia-smi on device %d: %v", devIndex, err)
	}

	output := string(out)

	// Extract Driver Version
	driverRe := regexp.MustCompile(`Driver Version\s*:\s*(\S+)`)
	driverMatches := driverRe.FindStringSubmatch(output)
	if len(driverMatches) < 2 {
		return fmt.Errorf("driver version not found in nvidia-smi output")
	}
	s.DriverVersion = driverMatches[1]

	// Extract CUDA Version
	cudaRe := regexp.MustCompile(`CUDA Version\s*:\s*(\S+)`)
	cudaMatches := cudaRe.FindStringSubmatch(output)
	if len(cudaMatches) < 2 {
		return fmt.Errorf("CUDA version not found in nvidia-smi output")
	}
	s.CUDAVersion = cudaMatches[1]

	return nil
}
