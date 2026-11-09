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

// CheckRdmaEnvService is the check-item key for the rdma-env-pre systemd unit check.
const CheckRdmaEnvService = "check_rdma_env_service"

// RdmaEnvCheckItems holds the template CheckerResult for each rdmaenv checker. The
// checker copies the template and fills in Status/Level/Curr/Detail: Level here is the
// worst case, and the checker downgrades it for the milder states.
var RdmaEnvCheckItems = map[string]common.CheckerResult{
	CheckRdmaEnvService: {
		Name:        CheckRdmaEnvService,
		Description: "Check that the rdma-env-pre systemd unit completed its RDMA NIC initialization",
		Spec:        "active (running or completed)",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "RdmaEnvServiceNotHealthy",
		Suggestion:  "Inspect `systemctl status rdma-env-pre` and /var/log/rdma-env-pre/init.log; RDMA NIC settings may be unapplied",
	},
}
