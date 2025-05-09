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
package perftest

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

const (
	NcclPerfCheckerName = "NcclPerfCheckerName"
)

// NcclPerfCheckItems is a map of check items for ncclperf
var NcclPerfCheckItems = map[string]*common.CheckerResult{
	NcclPerfCheckerName: {
		Name:        NcclPerfCheckerName,
		Description: "",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "NcclPerfError",
		Suggestion:  "Check Nccl Bandwidth",
	},
}
