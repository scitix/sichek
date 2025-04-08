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
	"time"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

type ComponentUserConfig interface {
	GetCheckerSpec() map[string]CheckerSpec
	GetQueryInterval() time.Duration
	GetComponentName() string
	LoadUserConfigFromYaml(file string) error
}
type ComponentSpecConfig interface {
	LoadSpecConfigFromYaml(file string) error
}

type CheckerSpec interface {
	// LoadFromYaml(file string) error
}

func GetDefaultConfigFiles(component string) (string, []os.DirEntry, error) {
	defaultCfgDirPath := filepath.Join(consts.DefaultPodCfgPath, component)
	_, err := os.Stat(defaultCfgDirPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return "", nil, fmt.Errorf("get curr file path failed")
		}
		// 获取当前文件的目录
		commonDir := filepath.Dir(curFile)
		defaultCfgDirPath = filepath.Join(filepath.Dir(commonDir), component, "config")
	}
	files, err := os.ReadDir(defaultCfgDirPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read directory: %v", err)
	}
	return defaultCfgDirPath, files, nil
}

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
