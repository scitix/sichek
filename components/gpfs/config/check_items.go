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

const (
	GPFSTimeClockCheckerName       = "time_clock"
	OSLockupCheckerName            = "OS_lockup"
	RDMACheckerName                = "RDMA"
	QuorumConnectionCheckerName    = "quorum_connection"
	TcpStateCheckerName            = "tcp_state"
	FilesystemUnmountCheckerName   = "filesystem_unmount"
	ExpelledFromClusterCheckerName = "expelled_from_cluster"
	UnauthorizedCheckerName        = "unauthorized"
	Bond0Lost					   = "bond0_lost"
)

var GPFSCheckItems = map[string]common.CheckerResult{
	GPFSTimeClockCheckerName: {
		Name:        GPFSTimeClockCheckerName,
		Description: "Time-of-day may have jumped back",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "TimeClockError",
		Suggestion:  "Reset the time clock with ntp",
	},
	OSLockupCheckerName: {
		Name:        OSLockupCheckerName,
		Description: "OS lockup, may cause GPFS heartbeat fail and unmount",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "OSLockup",
		Suggestion:  "Fix OS kernel and driver bugs",
	},
	RDMACheckerName: {
		Name:        RDMACheckerName,
		Description: "node RDMA network down",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "RDMAStatusError",
		Suggestion:  "Check RDMA network and device",
	},
	QuorumConnectionCheckerName: {
		Name:        QuorumConnectionCheckerName,
		Description: "connection with quorum node down",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "QuorumConnectionDown",
		Suggestion:  "Check GPFS daemon network",
	},
	TcpStateCheckerName: {
		Name:        TcpStateCheckerName,
		Description: "node TCP connection down",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "BadTcpState",
		Suggestion:  "Check GPFS daemon network",
	},
	FilesystemUnmountCheckerName: {
		Name:        FilesystemUnmountCheckerName,
		Description: "node filesystem unmounted",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPFSUnmount",
		Suggestion:  "Check GPFS status",
	},
	ExpelledFromClusterCheckerName: {
		Name:        ExpelledFromClusterCheckerName,
		Description: "node expelled from GPFS cluster",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "ExpelledFromGPFSCluster",
		Suggestion:  "Check GPFS daemon network and status",
	},
	UnauthorizedCheckerName: {
		Name:        UnauthorizedCheckerName,
		Description: "node unauthorized for remote cluster",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "GPFSUnauthorized",
		Suggestion:  "Check GPFS authorization status",
	},
	UnauthorizedCheckerName: {
		Name:        Bond0Lost,
		Description: "bond0 not active",
		Device:      "",
		Spec:        "0",
		Status:      "",
		Level:       consts.LevelWarning,
		Detail:      "",
		ErrorName:   "Bond0Lost",
		Suggestion:  "Check GPFS ether network",
	},
}
