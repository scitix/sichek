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
	LoadUserConfigFromYaml(file string) error
}
type ComponentSpecConfig interface {
	LoadSpecConfigFromYaml(file string) error
}

type CheckerSpec interface {
	// LoadFromYaml(file string) error
}

func DefaultComponentConfig(component string, config interface{}, filename string) error {
	defaultCfgPath := filepath.Join(consts.DefaultPodCfgPath, component, filename)
	_, err := os.Stat(defaultCfgPath)
	if err != nil {
		// run on host use local config
		_, curFile, _, ok := runtime.Caller(0)
		if !ok {
			return fmt.Errorf("get curr file path failed")
		}
		commonDir := filepath.Dir(curFile)
		// 获取当前文件的目录
		defaultCfgPath = filepath.Join(filepath.Dir(commonDir), component, "config", filename)
	}
	err = utils.LoadFromYaml(defaultCfgPath, config)
	return err
}
