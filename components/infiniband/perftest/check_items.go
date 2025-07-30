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
	IBPerfTestName   = "IBPerfTest"
	NCCLPerfTestName = "NCCLPerfTest"
)

// PerfCheckItems is a map of check items for ibperf and ncclperf
var PerfCheckItems = map[string]*common.CheckerResult{
	IBPerfTestName: {
		Name:        IBPerfTestName,
		Description: "",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "IbPerfTestError",
		Suggestion:  "Check Ib Bandwidth",
	},
	NCCLPerfTestName: {
		Name:        NCCLPerfTestName,
		Description: "",
		Status:      consts.StatusNormal,
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "NcclPerfTestError",
		Suggestion:  "Check Nccl Bandwidth",
	},
}
