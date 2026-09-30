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

import (
	"time"

	"github.com/scitix/sichek/components/common"
)

type LldpUserConfig struct {
	LLDP *LldpConfig `yaml:"lldp"`
}

type LldpConfig struct {
	QueryInterval common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize     int64           `json:"cache_size"     yaml:"cache_size"`
	// LldpctlPath overrides the lldpctl binary location. Empty = look up in $PATH.
	LldpctlPath string `json:"lldpctl_path" yaml:"lldpctl_path"`
	// ExecTimeout caps each lldpctl invocation. Default 10s.
	ExecTimeout common.Duration `json:"exec_timeout" yaml:"exec_timeout"`
}

func (c *LldpUserConfig) GetQueryInterval() common.Duration {
	if c.LLDP == nil || c.LLDP.QueryInterval.Duration == 0 {
		return common.Duration{Duration: 5 * time.Minute}
	}
	return c.LLDP.QueryInterval
}

func (c *LldpUserConfig) SetQueryInterval(newInterval common.Duration) {
	if c.LLDP == nil {
		c.LLDP = &LldpConfig{}
	}
	c.LLDP.QueryInterval = newInterval
}
