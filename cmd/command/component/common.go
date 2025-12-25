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
package component

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
)

var (
	ComponentStatuses = make(map[string]bool) // Tracks pass/fail status for each component
	StatusMutex       sync.Mutex              // Ensures thread-safe updates
)

type CheckResults struct {
	component common.Component
	result    *common.Result
	info      common.Info
}

func RunComponentCheck(ctx context.Context, comp common.Component, timeout time.Duration) (*CheckResults, error) {
	result, err := common.RunHealthCheckWithTimeout(ctx, timeout, comp.Name(), comp.HealthCheck)
	if err != nil {
		logrus.WithField("component", comp.Name()).Error(err) // Updated to use comp.Name()
		return nil, err
	}

	info, err := comp.LastInfo()
	if err != nil && (comp.Name() != consts.ComponentNameSyslog && comp.Name() != consts.ComponentNamePodlog) {
		logrus.WithField("component", comp.Name()).Errorf("failed to get the LastInfo: %v", err) // Updated to use comp.Name()
		return nil, err
	}
	return &CheckResults{
		component: comp,
		result:    result,
		info:      info,
	}, nil
}

func PrintCheckResults(summaryPrint bool, checkResult *CheckResults) {
	passed := checkResult.component.PrintInfo(checkResult.info, checkResult.result, summaryPrint)
	StatusMutex.Lock()
	ComponentStatuses[checkResult.component.Name()] = passed
	StatusMutex.Unlock()
}

// GetComponentsFromConfig extracts component names from default_user_config.yaml.
// It returns only components with enable=true (excluding "metrics").
func GetComponentsFromConfig(cfgFile string) ([]string, error) {
	var config map[string]interface{}
	err := common.LoadUserConfig(cfgFile, &config)
	if err != nil {
		return nil, err
	}

	components := []string{}
	for key, value := range config {
		// Skip "metrics" as it's not a component
		if key == "metrics" {
			continue
		}
		// Check if this is a component config (should be a map)
		_, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		components = append(components, key)
	}
	return components, nil
}

// DetermineComponentsToCheck determines which components to check based on enable-components flag,
// ignore-components flag, and the configuration file.
// Parameters:
//   - enableComponents: comma-separated list of components to enable (from -E flag), empty string means use config
//   - ignoredComponents: list of components to ignore (from -I flag)
//   - cfgFile: path to the user config file
//   - logField: field name for logging (e.g., "all" or "daemon")
//
// Returns the list of component names to check.
func DetermineComponentsToCheck(enableComponents string, ignoredComponents string, cfgFile string, logField string) []string {
	var componentsToCheck []string
	if cfgFile == "" {
		cfgFile = filepath.Join(consts.DefaultProductionCfgPath, consts.DefaultUserCfgName)
	}
	if len(enableComponents) > 0 {
		usedComponents := []string{}
		for _, comp := range slices.Compact(strings.Split(enableComponents, ",")) {
			if comp != "" {
				usedComponents = append(usedComponents, strings.TrimSpace(comp))
			}
		}
		componentsToCheck = usedComponents
		logrus.WithField(logField, logField).Infof("using enabled components from -E flag: %v", componentsToCheck)
	} else {
		// Otherwise, load components from default_user_config.yaml and exclude -I components
		configComponents, err := GetComponentsFromConfig(cfgFile)
		if err != nil {
			logrus.WithField(logField, logField).Warnf("failed to load components from config, falling back to DefaultComponents: %v", err)
			componentsToCheck = consts.DefaultComponents
		}
		// Filter out ignored components
		ignoredComponentsList := []string{}
		if len(ignoredComponents) > 0 {
			ignoredComponentsList = strings.Split(ignoredComponents, ",")
		}
		for _, comp := range configComponents {
			if !slices.Contains(ignoredComponentsList, comp) {
				componentsToCheck = append(componentsToCheck, comp)
			}
		}
		logrus.WithField(logField, logField).Infof("using components from config (excluding -I): %v", componentsToCheck)
	}
	return componentsToCheck
}
