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
	ComponentNameGpuEvents  = "gpuevents"
	ComponentIDPodLog       = "12"
	ComponentNamePodlog     = "podlog"
	ComponentIDHCA          = "13"
	ComponentNameHCA        = "hca"
	ComponentIDPCIE         = "14"
	ComponentNamePCIE       = "pcie"
	ComponentIDSyslog       = "15"
	ComponentNameSyslog     = "syslog"

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
	CheckerIDPodLog                  = "4300"
	CheckerIDHang                    = "4400"

	/*----------------------error name------------------------*/
	ErrorNameNCCL  = "NCCLTimeout"
	ErrorNameDmesg = "DmesgError"
)

const (
	KubeConfigPath = "/etc/kubernetes/kubelet.conf"
	DefaultAnnoKey = "scitix.ai/sichek"

	ServiceName = "sichek.service"
)

var (
	DefaultVersion                = "v1"
	DefaultComponentQueryInterval = time.Duration.Seconds(1)

	DefaultComponents = []string{
		ComponentNameCPU, ComponentNameNvidia, ComponentNameInfiniband, ComponentNameGpfs, ComponentNameDmesg,
		ComponentNamePodlog, ComponentNameGpuEvents, ComponentNameSyslog,
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

// priority map
var LevelPriority = map[string]int{
	LevelInfo:     1,
	LevelWarning:  2,
	LevelCritical: 3,
	LevelFatal:    4,
}

const (
	DefaultUserCfgName       = "default_user_config.yaml"
	DefaultSpecCfgName       = "default_spec.yaml"
	DefaultSpecSuffix        = "_spec.yaml"
	DefaultEventRuleName     = "default_event_rules.yaml"
	DefaultEventRuleSuffix   = "_rules.yaml"
	DefaultProductionPath    = "/var/sichek"
	DefaultProductionCfgPath = "/var/sichek/config"
	DefaultOssCfgPath        = "https://oss-ap-southeast.scitix.ai/hisys-sichek/specs"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Purple = "\033[35m"
	Cyan   = "\033[36m"
	White  = "\033[37m"
)
const PadLen = len(Green) + len(Reset)
const CmdTimeout = 30 * time.Second
const IbPerfTestTimeout = 600 * time.Second
const AllCmdTimeout = 60 * time.Second
const DefaultCacheLine int64 = 10000              // Default cache line number for event filter
const DefaultFileLoaderInterval = 5 * time.Second // Default interval for file loader scheduler
