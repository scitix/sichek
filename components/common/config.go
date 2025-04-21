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
package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ComponentUserConfig defines the methods for getting and setting user configuration.
type ComponentUserConfig interface {
	GetQueryInterval() time.Duration
	SetQueryInterval(newInterval time.Duration)
}

// ComponentSpecConfig defines the method for loading specification config from YAML.
type ComponentSpecConfig interface {
	LoadSpecConfigFromYaml(file string) error
}

type CheckerSpec interface {
	// LoadFromYaml(file string) error
}

// GetDefaultConfigFiles returns the default configuration directory and the files inside it.
func GetDefaultConfigFiles(component string) (string, []os.DirEntry, error) {
	defaultCfgDirPath := filepath.Join(consts.DefaultPodCfgPath, component)
	_, err := os.Stat(defaultCfgDirPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return "", nil, fmt.Errorf("get curr file path failed")
		}
		// Get the directory of the current file
		commonDir := filepath.Dir(curFile)
		defaultCfgDirPath = filepath.Join(filepath.Dir(commonDir), component, "config")
	}
	files, err := os.ReadDir(defaultCfgDirPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read directory: %v", err)
	}
	return defaultCfgDirPath, files, nil
}

// DefaultComponentConfig loads the default configuration for a given component from a YAML file.
func DefaultComponentConfig(component string, config interface{}, filename string) error {
	defaultCfgDirPath, files, err := GetDefaultConfigFiles(component)
	if err != nil {
		return fmt.Errorf("failed to get default config files: %v", err)
	}
	for _, file := range files {
		if file.Name() == filename {
			defaultCfgPath := filepath.Join(defaultCfgDirPath, file.Name())
			err = utils.LoadFromYaml(defaultCfgPath, config)
			return err
		}
	}
	return fmt.Errorf("failed to find default config file: %s", filename)
}

// DefaultComponentConfig loads the default configuration for a given component from a YAML file.
func LoadComponentUserConfig(file string, config interface{}) error {
	if file != "" {
		err := utils.LoadFromYaml(file, config)
		if err == nil {
			return nil
		}
	}
	defaultCfgPath := filepath.Join(consts.DefaultPodCfgPath, "config", consts.DefaultUserCfgName)
	_, err := os.Stat(defaultCfgPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		// Get the directory of the current file
		commonDir := filepath.Dir(curFile)
		sichekDir := filepath.Dir(filepath.Dir(commonDir))

		defaultCfgPath = filepath.Join(sichekDir, "config", consts.DefaultUserCfgName)
	}
	err = utils.LoadFromYaml(defaultCfgPath, config)
	return err

}

// FreqController controls the frequency of component queries.
type FreqController struct {
	mu      sync.Mutex
	modules map[string]ComponentUserConfig
}

// RegisterModule registers a new module with its configuration.
func (fc *FreqController) RegisterModule(moduleName string, moduleCfg ComponentUserConfig) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.modules[moduleName] = moduleCfg
}

// SetModuleQueryInterval sets the query interval for a specific module.
func (fc *FreqController) SetModuleQueryInterval(moduleName string, newInterval time.Duration) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if module, exists := fc.modules[moduleName]; exists {
		module.SetQueryInterval(newInterval)
	}
}

// GetModuleQueryInterval retrieves the query interval for a specific module.
func (fc *FreqController) GetModuleQueryInterval(moduleName string) time.Duration {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if module, exists := fc.modules[moduleName]; exists {
		return module.GetQueryInterval()
	} else {
		// If the module is not registered, return a default value.
		logrus.WithField("component", "FreqController").Errorf("module %s not registered", moduleName)
		return 0
	}
}

// Global instance for the frequency controller.
var (
	freqController     *FreqController
	freqControllerOnce sync.Once
)

// GetFreqController creates and returns a singleton instance of FreqController.
func GetFreqController() *FreqController {
	freqControllerOnce.Do(func() {
		freqController = &FreqController{
			modules: make(map[string]ComponentUserConfig),
		}
	})
	return freqController
}
