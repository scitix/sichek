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
package infiniband_config

import (
	"time"

	"github.com/scitix/sichek/components/common"
)

// 实现ComponentsConfig 接口
type InfinibandConfig struct {
	Name            string        `json:"name" yaml:"name"`
	QueryInterval   time.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64         `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers []string      `json:"ignored_checkers" yaml:"ignored_checkers"`
}

func (c *InfinibandConfig) GetCheckerSpec() map[string]common.CheckerSpec {
	return nil
}

func (c *InfinibandConfig) GetQueryInterval() time.Duration {
	return c.QueryInterval
}
