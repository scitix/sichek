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
	"encoding/json"
	"time"

	"github.com/scitix/sichek/components/common"
)

type GPFSInfo struct {
	Time            time.Time		`json:"time"`
	EventInfo 		EventInfo		`json:"event_info"`
	XStorHealthInfo	XStorHealthInfo	`json:"xstor_health_info`
}

func (i *GPFSInfo) JSON() (string, error) {
	data, err := json.Marshal(i)
	return string(data), err
}

// ToString Convert struct to JSON (pretty-printed)
func (i *GPFSInfo) ToString() string {
	return common.ToString(i)
}
