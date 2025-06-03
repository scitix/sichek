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
package collector

import (
	"github.com/scitix/sichek/components/common"
)

type GPFSXStorHealthItem struct {
	Item		string			`json:"item" yaml:"item"`
	Node		string			`json:"node" yaml:"node"`
	Dev			string			`json:"dev" yaml:"dev"`
	Detail		string			`json:"detail" yaml:"detail"`
	Spec		string			`json:"spec" yaml:"spec"`
	Curr		string			`json:"curr" yaml:"curr"`
	Status		string			`json:"status" yaml:"status"`
	Info		string			`json:"info" yaml:"info"`
	Errno		uint64			`json:"errno" yaml:"errno"`
}

type XStorHealthInfo struct {
	HealthItems map[string]*GPFSXStorHealthItem
}

func (xstorHealthInfo *XStorHealthInfo) JSON() ([]byte, error) {
	return common.JSON(xstorHealthInfo)
}

func (xstorHealthInfo *XStorHealthInfo) ToString() string {
	return common.ToString(xstorHealthInfo)
}
