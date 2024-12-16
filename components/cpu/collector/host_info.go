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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/scitix/sichek/components/common"
)

type HostInfo struct {
	Hostname      string `json:"hostname"`
	OSVersion     string `json:"os_version"`
	KernelVersion string `json:"kernel_version"`
}

// JSON converts the HostInfo struct to a JSON byte slice.
func (hostInfo *HostInfo) JSON() ([]byte, error) {
	return common.JSON(hostInfo)
}

// ToString converts the HostInfo struct to a pretty-printed JSON string.
func (hostInfo *HostInfo) ToString() string {
	return common.ToString(hostInfo)
}

// Get retrieves the hostname, OS version, and kernel version of the host.
func (hostInfo *HostInfo) Get() error {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	hostInfo.Hostname = hostname

	// Get OS version (Linux-specific example)
	osVersion, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(osVersion), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME") {
				hostInfo.OSVersion = strings.Trim(strings.Split(line, "=")[1], `"`+"\n")
				break
			}
		}
	} else {
		hostInfo.OSVersion = "Unknown"
	}

	// Get kernel version
	kernelVersion, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return fmt.Errorf("failed to get kernel version: %w", err)
	}
	hostInfo.KernelVersion = strings.TrimSpace(string(kernelVersion))

	return nil
}
