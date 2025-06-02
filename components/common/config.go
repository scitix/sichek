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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

// ComponentUserConfig defines the methods for getting and setting user configuration.
type ComponentUserConfig interface {
	GetQueryInterval() Duration
	SetQueryInterval(newInterval Duration)
}

// ComponentSpecConfig defines the method for loading specification config from YAML.
type ComponentSpecConfig interface {
	LoadSpecConfigFromYaml(file string) error
}

type CheckerSpec interface {
	// LoadFromYaml(file string) error
}

// GetDevDefaultConfigFiles returns the default config dir and and extract default spec for the given component.
// for production environment, it checks the default config path (e.g., /var/sichek/config/xx-component).
// for development environment, it checks the default config path based on runtime.Caller  (e.g., /repo/component/xx-component/config).
func GetDevDefaultConfigFiles(component string) (string, []os.DirEntry, error) {
	// Try production path first: /var/sichek/config/xx-component
	defaultCfgDirPath := filepath.Join(consts.DefaultProductionCfgPath, component)
	_, err := os.Stat(defaultCfgDirPath)
	if err != nil {
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return "", nil, fmt.Errorf("get curr file path failed")
		}
		// Locate current file: /repo/component/xx-component/config
		commonDir := filepath.Dir(curFile)
		defaultCfgDirPath = filepath.Join(filepath.Dir(commonDir), component, "config")
	}
	files, err := os.ReadDir(defaultCfgDirPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read directory: %v", err)
	}
	return defaultCfgDirPath, files, nil
}

// LoadFromProductionDefaultSpec checks and extract default spec from production env.
func LoadFromProductionDefaultSpec(spec interface{}) error {
	defaultProductionCfgPath := filepath.Join(consts.DefaultProductionCfgPath, "config", consts.DefaultSpecCfgName)
	_, err := os.Stat(defaultProductionCfgPath)
	if err != nil {
		return fmt.Errorf("production config path not found: %w", err)
	}
	err = utils.LoadFromYaml(defaultProductionCfgPath, spec)
	if err != nil {
		return fmt.Errorf("failed to read production config spec %s: %w", defaultProductionCfgPath, err)
	}
	return nil
}

// LoadFromOssSpec downloads and parses a YAML spec from a given URL into the provided spec structure.
func LoadSpecFromOss(url string, spec interface{}) error {
	if url == "" {
		return fmt.Errorf("url is empty")
	}
	if !(len(url) >= 7 && (url[:7] == "http://" || url[:8] == "https://")) {
		return fmt.Errorf("unsupported URL scheme (must start with http:// or https://): %s", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch spec from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %d while fetching %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read body from %s: %w", url, err)
	}

	if err := yaml.Unmarshal(data, spec); err != nil {
		return fmt.Errorf("failed to unmarshal YAML from %s: %w", url, err)
	}

	return nil
}

// LoadUserConfig loads the default user config from production default dir or dev default dir based on runtime.Caller
func LoadDefaultEventRules(eventRule interface{}, component string) error {
	// 1. try to load default config from production default dir
	defaultEventRuleCfg := filepath.Join(consts.DefaultProductionCfgPath, component, consts.DefaultEventRuleName)
	_, err := os.Stat(defaultEventRuleCfg)
	if err == nil {
		err := utils.LoadFromYaml(defaultEventRuleCfg, eventRule)
		if err == nil {
			return nil
		}
	}
	// 2. try to load default config from default config directory based on caller path
	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("get curr file path failed")
	}
	// Get the directory of the current file
	commonDir := filepath.Dir(curFile)
	defaultEventRuleCfg = filepath.Join(filepath.Dir(commonDir), component, "config", consts.DefaultEventRuleName)
	err = utils.LoadFromYaml(defaultEventRuleCfg, eventRule)
	return err
}

// LoadUserConfig loads the user config from provided file or production default dir or dev default dir based on runtime.Caller
func LoadUserConfig(file string, config interface{}) error {
	// 1. Load config from provided file
	if file != "" {
		err := utils.LoadFromYaml(file, config)
		if err == nil {
			return nil
		}
	}
	// 2. try to load default config from production env if no file specified
	defaultUserCfg := filepath.Join(consts.DefaultProductionCfgPath, consts.DefaultUserCfgName)
	_, err := os.Stat(defaultUserCfg)
	if err == nil {
		err := utils.LoadFromYaml(defaultUserCfg, config)
		if err == nil {
			return nil
		}
	}
	// 3. try to load default spec from default config directory based on caller path
	_, curFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("get curr file path failed")
	}
	// Get the directory of the current file
	commonDir := filepath.Dir(curFile)
	sichekDir := filepath.Dir(filepath.Dir(commonDir))

	defaultUserCfg = filepath.Join(sichekDir, "config", consts.DefaultUserCfgName)
	err = utils.LoadFromYaml(defaultUserCfg, config)
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
func (fc *FreqController) SetModuleQueryInterval(moduleName string, newInterval Duration) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if module, exists := fc.modules[moduleName]; exists {
		module.SetQueryInterval(newInterval)
	}
}

// GetModuleQueryInterval retrieves the query interval for a specific module.
func (fc *FreqController) GetModuleQueryInterval(moduleName string) Duration {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if module, exists := fc.modules[moduleName]; exists {
		return module.GetQueryInterval()
	} else {
		// If the module is not registered, return a default value.
		logrus.WithField("component", "FreqController").Errorf("module %s not registered", moduleName)
		return Duration{0}
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
