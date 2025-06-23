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
	GPFSInstalledCheckerName   = "gpfs-installed"
	NodeInClusterCheckerName   = "node-in-cluster"
	GPFSStartedCheckerName     = "gpfs-started"
	GPFSMountedCheckerName     = "gpfs-mounted"
	GPFSHealthCheckerName      = "gpfs-health"
	GPFSRdmaNetworkCheckerName = "gpfs-rdma-network"
)

var GPFSCheckItems = map[string]common.CheckerResult{
	GPFSInstalledCheckerName: {
		Name:        GPFSInstalledCheckerName,
		Description: "Check if GPFS software installed",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPFSNotInstalled",
		Suggestion:  "Install GPFS software",
	},
	NodeInClusterCheckerName: {
		Name:        NodeInClusterCheckerName,
		Description: "Check if node is in GPFS cluster",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPFSNotInCluster",
		Suggestion:  "Add node to GPFS cluster",
	},
	GPFSStartedCheckerName: {
		Name:        GPFSStartedCheckerName,
		Description: "Check if GPFS software started",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPFSNotStarted",
		Suggestion:  "Start GPFS software",
	},
	GPFSMountedCheckerName: {
		Name:        GPFSMountedCheckerName,
		Description: "Check if GPFS mounted",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPFSNotMounted",
		Suggestion:  "Mount GPFS filesystem",
	},
	GPFSHealthCheckerName: {
		Name:        GPFSHealthCheckerName,
		Description: "Check if GPFS node is healthy",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPFSNodeNotHealthy",
		Suggestion:  "Check mmhealth and GPFS log for details",
	},
	GPFSRdmaNetworkCheckerName: {
		Name:        GPFSRdmaNetworkCheckerName,
		Description: "Check if GPFS using RDMA",
		Status:      "",
		Level:       consts.LevelCritical,
		Detail:      "",
		ErrorName:   "GPFSRDMAError",
		Suggestion:  "Check node RDMA network and GPFS log",
	},
}
