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
package consts

import "time"

const (
	/*-----------------conponent id && name-------------------*/
	ComponentIDCPU          = "01"
	ComponentNameCPU        = "cpu"
	ComponentIDMemory       = "02"
	ComponentNameMemory     = "memory"
	ComponentIDNvidia       = "03"
	ComponentNameNvidia     = "nvidia"
	ComponentIDInfiniband   = "04"
	ComponentNameInfiniband = "infiniband"
	ComponentIDEthernet     = "05"
	ComponentNameEthernet   = "ethernet"
	ComponentIDGpfs         = "07"
	ComponentNameGpfs       = "gpfs"
	ComponentIDDMesg        = "10"
	ComponentNameDmesg      = "dmesg"
	ComponentIDHang         = "11"
	ComponentNameHang       = "hang"
	ComponentIDNCCL         = "12"
	ComponentNameNCCL       = "nccl"
	ComponentIDHCA          = "13"
	ComponentNameHCA        = "hca"

	/*----------------------checker id------------------------*/
	CheckerIDInfinibandFW            = "4001"
	CheckerIDInfinibandNicNum        = "4002"
	CheckerIDInfinibandNicNetDev     = "4003"
	CheckerIDInfinibandPhyState      = "4004"
	CheckerIDInfinibandIBState       = "4005"
	CheckerIDInfinibandPCIEACS       = "4006"
	CheckerIDInfinibandPCIEMRR       = "4007"
	CheckerIDInfinibandPCIESpeed     = "4008"
	CheckerIDInfinibandPCIEWidth     = "4009"
	CheckerIDInfinibandPCIETreeSpeed = "4010"
	CheckerIDInfinibandPCIETreeWidth = "4011"
	CheckerIDEthPhyState             = "4111"
	CheckerIDInfinibandOFED          = "4012"
	CheckerIDInfinibandPortSpeed     = "4013"
	CheckerNetOperstate              = "4014"
	CheckerIDDmesg                   = "4200"
	CheckerIDNCCL                    = "4300"
	CheckerIDHang                    = "4400"

	/*----------------------error name------------------------*/
	ErrorNameHang  = "GPUHang"
	ErrorNameNCCL  = "NCCLTimeout"
	ErrorNameDmesg = "DmesgError"
)

const (
	TaskGuardEndpoint = "localhost"
	TaskGuardPort     = 15040

	KubeConfigPath = "/etc/kubernetes/kubelet.conf"
	DefaultAnnoKey = "scitix.ai/sichek"

	ServiceName = "sichek.service"
)

var (
	DefaultVersion                = "v1"
	DefaultComponentQueryInterval = time.Duration.Seconds(1)

	DefaultComponents = []string{
		ComponentNameCPU, ComponentNameNvidia, ComponentNameInfiniband, ComponentNameGpfs, // ComponentNameDmesg,
		ComponentNameNCCL, ComponentNameHang,
	}
)

const (
	/*---------------component&checker result level---------------*/
	LevelInfo     = "info"
	LevelWarning  = "warning"
	LevelCritical = "critical"
	LevelFatal    = "fatal"

	/*----------------------component status----------------------*/
	StatusNormal   = "normal"
	StatusAbnormal = "abnormal"
)

const (
	DefaultBasicCfgName = "/default_user_config.yaml"
	DefaultSpecCfgName  = "/default_spec_config.yaml"
	DefaultPodCfgPath   = "/var/sichek11/"
)
