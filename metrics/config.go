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
package metrics

import (
	"fmt"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/pkg/utils"
)

type MetricsUserConfig struct {
	Metrics *MetricsConfig `json:"metrics" yaml:"metrics"`
}

type MetricsConfig struct {
	Port              int      `json:"port" yaml:"port"`
}

func (c *MetricsUserConfig) LoadUserConfigFromYaml(file string) error {
	if file == "" {
		return common.DefaultComponentUserConfig(c)
	}
	err := utils.LoadFromYaml(file, c)
	if err != nil || c.Metrics == nil {
		return fmt.Errorf("failed to load metrics config: %v", err)
	}
	return nil
}
