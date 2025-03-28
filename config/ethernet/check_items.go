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
package ethernet

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

var (
	ChekEthPhyState = "eth_phy_tate"

	checkDes = map[string]string{
		ChekEthPhyState: "check the eth phy state",
	}
	checkLevel = map[string]string{
		ChekEthPhyState: consts.LevelCritical,
	}
	errName = map[string]string{
		ChekEthPhyState: "EthPhySate",
	}
	checkAction = map[string]string{
		ChekEthPhyState: "the eth link is not link up, check link status",
	}
	checkDetail = map[string]string{
		ChekEthPhyState: "the phy state is up",
	}
)

var EthCheckItems = map[string]common.CheckerResult{
	ChekEthPhyState: {
		Name:        ChekEthPhyState,
		Description: checkDes[ChekEthPhyState],
		Status:      "",
		Level:       checkLevel[ChekEthPhyState],
		Detail:      checkDetail[ChekEthPhyState],
		ErrorName:   errName[ChekEthPhyState],
		Suggestion:  checkAction[ChekEthPhyState],
	},
}
