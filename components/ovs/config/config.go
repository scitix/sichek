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
package config

import "github.com/scitix/sichek/components/common"

const (
	ServiceCheckerName     = "ovs_service"
	VersionCheckerName     = "ovs_version"
	OtherConfigCheckerName = "ovs_other_config"
	BridgeCheckerName      = "ovs_bridge"
)

type OVSUserConfig struct {
	OVS *OVSConfig `json:"ovs" yaml:"ovs"`
}

type OVSConfig struct {
	QueryInterval   common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64           `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers []string        `json:"ignored_checkers" yaml:"ignored_checkers"`
	EnableMetrics   bool            `json:"enable_metrics" yaml:"enable_metrics"`
}

func (c *OVSUserConfig) GetQueryInterval() common.Duration {
	if c.OVS == nil {
		return common.Duration{}
	}
	return c.OVS.QueryInterval
}

func (c *OVSUserConfig) SetQueryInterval(newInterval common.Duration) {
	if c.OVS == nil {
		c.OVS = &OVSConfig{}
	}
	c.OVS.QueryInterval = newInterval
}
