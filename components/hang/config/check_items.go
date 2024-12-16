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
	commonCfg "github.com/scitix/sichek/config"
)

// HangCheckItems is a map of check items for NCCL
var HangCheckItems = map[string]common.CheckerResult{
	"GPUHang": {
		Name:        "GPUHang",
		Description: "GPU probably hang",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       commonCfg.LevelFatal,
		Detail:      "",
		ErrorName:   "GPUHang",
		Suggestion:  "",
	},
}
