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
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
)

type MemoryInfo struct {
	// Total memory, in bytes
	MemTotal int `json:"mem_total"`
	// Total memory used, in bytes
	MemUsed int `json:"mem_used"`
	// Total memory free, in bytes
	MemFree int `json:"mem_free"`
	// The percentage of memory used.
	MemPercentUsed int `json:"mem_percent_used"`
	// Anonymous memory usage, in Bytes.
	MemAnonymousUsed int `json:"mem_anonymous_used"`
	// Page cache memory usage, in Bytes.
	PageCacheUsed int `json:"pagecache_used"`
	// Memory marked as unevictable usage, in Bytes.
	MemUnevictableUsed int `json:"mem_unevictable_used"`
	// Dirty pages usage, in Bytes
	DirtyPageUsed int `json:"dirty_page_used"`
}

func (memInfo *MemoryInfo) JSON() ([]byte, error) {
	return common.JSON(memInfo)
}

// Convert struct to JSON (pretty-printed)
func (memInfo *MemoryInfo) ToString() string {
	return common.ToString(memInfo)
}

func (memInfo *MemoryInfo) Get() error {
	return memInfo.get("/proc/meminfo")
}

func (memInfo *MemoryInfo) get(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read %v: %w", filename, err)
	}

	memInfoMap := make(map[string]int)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.Atoi(fields[1]) // Value is in KB
		if err != nil {
			return fmt.Errorf("failed to parse value for %s: %w", key, err)
		}
		memInfoMap[key] = value * 1024 // Convert from kB to bytes
	}

	memInfo.MemTotal = memInfoMap["MemTotal"]
	memInfo.MemFree = memInfoMap["MemFree"]
	memInfo.MemUsed = memInfo.MemTotal - memInfo.MemFree
	memInfo.MemPercentUsed = (memInfo.MemUsed * 100) / memInfo.MemTotal
	memInfo.MemAnonymousUsed = memInfoMap["AnonPages"]
	memInfo.PageCacheUsed = memInfoMap["Cached"]
	memInfo.MemUnevictableUsed = memInfoMap["Unevictable"]
	memInfo.DirtyPageUsed = memInfoMap["Dirty"]

	return nil
}
