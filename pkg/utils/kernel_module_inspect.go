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
package utils

import (
	"fmt"
	"os"
	"strings"
)

// IsKernalModuleLoaded checks if a specific kernel module is loaded
func IsKernalModuleLoaded(moduleName string) (bool, error) {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false, fmt.Errorf("failed to read /proc/modules: %w", err)
	}

	return strings.Contains(string(data), moduleName), nil
}

// IsKernalModuleHolder checks if a specific module is holding another module
func IsKernalModuleHolder(holder, module string) (bool, error) {
	path := fmt.Sprintf("/sys/module/%s/holders", holder)
	files, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Holder module or path does not exist")
			return false, nil // Holder module or path does not exist
		}
		return false, fmt.Errorf("failed to read holders for %s: %w", holder, err)
	}

	for _, file := range files {
		// fmt.Println(file.Name())
		if file.Name() == module {
			return true, nil
		}
	}

	return false, nil
}

// HasIOMMUGroups checks if IOMMU groups are present in /sys/kernel/iommu_groups
func HasIOMMUGroups() (bool, error) {
	const iommuPath = "/sys/kernel/iommu_groups"

	// Check if the path exists
	_, err := os.Stat(iommuPath)
	if os.IsNotExist(err) {
		return false, nil // IOMMU is likely disabled
	} else if err != nil {
		return false, fmt.Errorf("failed to access IOMMU groups: %w", err)
	}

	// Check if there are subdirectories (groups)
	groups, err := os.ReadDir(iommuPath)
	if err != nil {
		return false, fmt.Errorf("failed to read IOMMU groups: %w", err)
	}

	return len(groups) > 0, nil
}
