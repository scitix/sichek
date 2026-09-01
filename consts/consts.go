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
	ComponentIDSyslog         = "15"
	ComponentNameSyslog       = "syslog"
	ComponentIDTransceiver    = "16"
	ComponentNameTransceiver  = "transceiver"
	ComponentIDLLDP           = "17"
	ComponentNameLLDP         = "lldp"
	ComponentIDOVS            = "18"
	ComponentNameOVS          = "ovs"
	ComponentNameSysinfo      = "sysinfo"
	ComponentNameGPUProbe     = "gpuprobe"
	ComponentNameRdmaEnv      = "rdmaenv"

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

	// CPU extended checker IDs
	CheckerIDClockSyncService  = "1300"
	CheckerIDClockSyncOffset   = "1301"
	CheckerIDCPUMCEUncorrected = "1302"
	CheckerIDCPUMCECorrected   = "1303"

	// Memory extended checker IDs
	CheckerIDMemoryECCUncorrected = "2100"
	CheckerIDMemoryECCCorrected   = "2101"
	CheckerIDMemoryCapacity       = "2102"

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
		ComponentNameCPU, ComponentNameNvidia, ComponentNameInfiniband, ComponentNameEthernet, ComponentNameGpfs, ComponentNameDmesg,
		ComponentNamePodlog, ComponentNameGpuEvents, ComponentNameSyslog, ComponentNameTransceiver, ComponentNameLLDP,
		ComponentNameOVS, ComponentNameSysinfo,
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
	DefaultSnapshotPath      = "/var/sichek/data/snapshot.json"

	// OSS Spec URLs
	DomesticSpecURL = "https://oss-cn-shanghai-2.siflow.cn/hisys:hisys-sichek-sh/specs"
	OverseasSpecURL = "https://oss-ap-southeast.scitix.ai/hisys-sichek/specs"
)

// sysinfo (OS/host KV-script collector) defaults
const (
	DefaultSysinfoQueryInterval = 24 * time.Hour
	DefaultSysinfoTimeout       = 60 * time.Second
	DefaultSysinfoScriptPath    = "scripts/os/collect-config.sh"
	// DomesticScriptBaseURL is the last-resort base when SICHEK_SPEC_URL is
	// unavailable; it is DomesticSpecURL with the trailing "/specs" stripped.
	DomesticScriptBaseURL = "https://oss-cn-shanghai-2.siflow.cn/hisys:hisys-sichek-sh"
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
// LevelColor returns the ANSI color for a given severity level.
func LevelColor(level string) string {
	switch level {
	case LevelWarning:
		return Yellow
	case LevelCritical, LevelFatal:
		return Red
	default:
		return Green
	}
}

const PadLen = len(Green) + len(Reset)
const CmdTimeout = 30 * time.Second

// TransceiverCmdTimeout bounds a one-shot `sichek transceiver` run. It is larger
// than CmdTimeout because DOM data comes from mlxlink, which costs roughly a
// second per port and serializes on an MFT-wide lock, so the sweep scales with
// port count rather than running concurrently. A multi-plane node exposes each
// rail NIC as four PCIe functions: measured end to end at ~48s for 34 ports
// while the daemon was sweeping the same devices, which does not fit in 30s and
// used to surface as a bogus "transceiver: FAIL" with no data at all.
//
// It is deliberately a separate constant rather than a bump to CmdTimeout, which
// every other component shares and none of them need widened.
const TransceiverCmdTimeout = 120 * time.Second

// NvidiaSWInfoProbeTimeout bounds the `nvidia-smi -q` driver/CUDA-version probe
// run during NVIDIA collector initialization so a hung nvidia-smi (e.g. an
// NVLink fault stalling the query path) cannot block daemon startup for the full
// 30s CmdTimeout. On timeout the NVIDIA component reports a Critical InitError
// instead of holding the systemd "activating" state open indefinitely. This is a
// bound, not the loop fix: the DaemonSet keepalive tolerating the "activating"
// state is what actually prevents the start/kill loop, so this value only needs
// to keep startup reasonably prompt.
const NvidiaSWInfoProbeTimeout = 9 * time.Second

// NvidiaSMITimeoutErrName is the CheckerResult Name/ErrorName reported when the
// nvidia-smi startup probe (SoftwareInfo.Get) exceeds NvidiaSWInfoProbeTimeout.
// It is kept distinct from the generic "InitError" so recovery automation can
// key on this specific "nvidia-smi query path is hung" condition (annotation
// error_name and the sichek_nvidia_<name> metric series both derive from it).
const NvidiaSMITimeoutErrName = "NvidiaSMITimeout"

const IbPerfTestTimeout = 600 * time.Second
const AllCmdTimeout = 60 * time.Second
const DefaultCacheLine int64 = 10000              // Default cache line number for event filter
const DefaultFileLoaderInterval = 5 * time.Second // Default interval for file loader scheduler
