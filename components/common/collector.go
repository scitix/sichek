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
	"context"
	"encoding/json"
	"fmt"
)

type Collector interface {
	Name() string
	GetCfg() ComponentUserConfig
	Collect(ctx context.Context) (Info, error)
}

type Info interface {
	JSON() (string, error)
}

// JSON Base function to convert any struct to JSON
func JSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// ToString Base function to convert any struct to a pretty-printed JSON string
func ToString(v interface{}) string {
	jsonData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("Error converting struct to JSON: %v\n", err)
		return ""
	}
	return string(jsonData)
}
