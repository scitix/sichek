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
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

type EthernetUserConfig struct {
	Ethernet *EthernetConfig `json:"ethernet" yaml:"ethernet"`
}

type EthernetConfig struct {
	QueryInterval   common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64           `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers []string        `json:"ignored_checkers" yaml:"ignored_checkers"`
}

func (c *EthernetUserConfig) GetQueryInterval() common.Duration {
	if c.Ethernet == nil {
		return common.Duration{}
	}
	return c.Ethernet.QueryInterval
}

func (c *EthernetUserConfig) SetQueryInterval(newInterval common.Duration) {
	if c.Ethernet == nil {
		c.Ethernet = &EthernetConfig{}
	}
	c.Ethernet.QueryInterval = newInterval
}

const (
	EthernetL1CheckerName = "L1(Physical Link)"
	EthernetL2CheckerName = "L2(Bond)"
	EthernetL3CheckerName = "L3(LACP)"
	EthernetL4CheckerName = "L4(ARP)"
	EthernetL5CheckerName = "L5(Routing)"
)

var EthernetCheckItems = map[string]string{
	EthernetL1CheckerName: "Check Layer 1 properties",
	EthernetL2CheckerName: "Check Layer 2 properties",
	EthernetL3CheckerName: "Check Layer 3 properties",
	EthernetL4CheckerName: "Check Layer 4 properties",
	EthernetL5CheckerName: "Check Layer 5 properties",
}

func LoadDefaultEventRules() (common.EventRuleGroup, error) {
	eventRules := make(common.EventRuleGroup)
	err := common.LoadDefaultEventRules(&eventRules, consts.ComponentNameEthernet)
	return eventRules, err
}
