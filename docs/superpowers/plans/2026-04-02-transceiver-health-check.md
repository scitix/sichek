# Transceiver Health Check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增独立 transceiver component，检查服务器上所有光模块（IB/RoCE/管理网）的健康状态，支持按网络类型分级告警。

**Architecture:** 遵循 sichek 标准三层模式（Collector → Checkers → Component）。Collector 用 `ethtool -m` 采集以太网光模块 DOM 数据，用 `mlxlink -d -m` 采集 IB 光模块数据。8 个 Checker 分别检查光功率、温度、电压、偏置电流、厂商、链路错误和在位状态，根据网络分类（business/management）使用不同严格度。

**Tech Stack:** Go 1.23, ethtool, mlxlink (MFT), sysfs

---

## File Structure

| 操作 | 文件路径 | 职责 |
|------|---------|------|
| Create | `components/transceiver/config/config.go` | UserConfig + checker 名称常量 |
| Create | `components/transceiver/config/spec.go` | Spec 结构体 + 加载 |
| Create | `components/transceiver/config/default_spec.yaml` | 默认 spec（阈值、认证厂商） |
| Create | `components/transceiver/collector/transceiver_info.go` | Info 结构体 + Collect() 入口 |
| Create | `components/transceiver/collector/ethtool_dom.go` | `ethtool -m` 输出解析 |
| Create | `components/transceiver/collector/mlxlink_dom.go` | `mlxlink -d -m` 输出解析 |
| Create | `components/transceiver/collector/utils.go` | 接口枚举、网络分类 |
| Create | `components/transceiver/checker/checker.go` | NewCheckers() 注册 |
| Create | `components/transceiver/checker/check_tx_power.go` | 发送光功率检查 |
| Create | `components/transceiver/checker/check_rx_power.go` | 接收光功率检查 |
| Create | `components/transceiver/checker/check_temperature.go` | 温度检查 |
| Create | `components/transceiver/checker/check_voltage.go` | 电压检查 |
| Create | `components/transceiver/checker/check_bias_current.go` | 偏置电流检查 |
| Create | `components/transceiver/checker/check_vendor.go` | 厂商认证检查 |
| Create | `components/transceiver/checker/check_link_errors.go` | 链路错误检查 |
| Create | `components/transceiver/checker/check_presence.go` | 光模块在位检查 |
| Create | `components/transceiver/transceiver.go` | Component 主体 |
| Create | `components/transceiver/metrics/metrics.go` | Prometheus 指标导出 |
| Create | `cmd/command/component/transceiver.go` | CLI 子命令 |
| Modify | `consts/consts.go` | 新增 ComponentNameTransceiver |
| Modify | `cmd/command/command.go` | 注册 transceiver 命令 |
| Modify | `cmd/command/component/all.go` | NewComponent switch 加 transceiver case |
| Modify | `config/default_user_config.yaml` | 追加 transceiver 配置段 |

---

### Task 1: 注册 consts + config 基础结构

**Files:**
- Modify: `consts/consts.go`
- Create: `components/transceiver/config/config.go`
- Create: `components/transceiver/config/spec.go`
- Create: `components/transceiver/config/default_spec.yaml`
- Modify: `config/default_user_config.yaml`

- [ ] **Step 1: 在 consts 中注册 transceiver**

在 `consts/consts.go` 的 component id/name 区域追加：

```go
ComponentIDTransceiver     = "16"
ComponentNameTransceiver   = "transceiver"
```

- [ ] **Step 2: 创建 config.go**

```go
// components/transceiver/config/config.go
package config

import "github.com/scitix/sichek/components/common"

const (
	TxPowerCheckerName    = "check_tx_power"
	RxPowerCheckerName    = "check_rx_power"
	TemperatureCheckerName = "check_temperature"
	VoltageCheckerName    = "check_voltage"
	BiasCurrentCheckerName = "check_bias_current"
	VendorCheckerName     = "check_vendor"
	LinkErrorsCheckerName = "check_link_errors"
	PresenceCheckerName   = "check_presence"
)

type TransceiverUserConfig struct {
	Transceiver *TransceiverConfig `json:"transceiver" yaml:"transceiver"`
}

type TransceiverConfig struct {
	QueryInterval   common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64           `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers []string        `json:"ignored_checkers" yaml:"ignored_checkers"`
	EnableMetrics   bool            `json:"enable_metrics" yaml:"enable_metrics"`
}

func (c *TransceiverUserConfig) GetQueryInterval() common.Duration {
	if c.Transceiver == nil {
		return common.Duration{}
	}
	return c.Transceiver.QueryInterval
}

func (c *TransceiverUserConfig) SetQueryInterval(newInterval common.Duration) {
	if c.Transceiver == nil {
		c.Transceiver = &TransceiverConfig{}
	}
	c.Transceiver.QueryInterval = newInterval
}
```

- [ ] **Step 3: 创建 spec.go**

```go
// components/transceiver/config/spec.go
package config

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

type TransceiverSpec struct {
	Networks map[string]*NetworkSpec `json:"networks" yaml:"networks"`
}

type NetworkSpec struct {
	InterfacePatterns []string        `json:"interface_patterns" yaml:"interface_patterns"`
	Thresholds        ThresholdSpec   `json:"thresholds" yaml:"thresholds"`
	CheckVendor       bool            `json:"check_vendor" yaml:"check_vendor"`
	CheckLinkErrors   bool            `json:"check_link_errors" yaml:"check_link_errors"`
	ApprovedVendors   []string        `json:"approved_vendors" yaml:"approved_vendors"`
}

type ThresholdSpec struct {
	TxPowerMarginDB      float64 `json:"tx_power_margin_db" yaml:"tx_power_margin_db"`
	RxPowerMarginDB      float64 `json:"rx_power_margin_db" yaml:"rx_power_margin_db"`
	TemperatureWarningC  float64 `json:"temperature_warning_c" yaml:"temperature_warning_c"`
	TemperatureCriticalC float64 `json:"temperature_critical_c" yaml:"temperature_critical_c"`
}

func LoadSpec(file string) (*TransceiverSpec, error) {
	spec := &TransceiverSpec{}

	// Try provided file
	if file != "" {
		if err := utils.LoadFromYaml(file, spec); err == nil && spec.Networks != nil {
			return spec, nil
		}
	}

	// Try production path
	if err := common.LoadSpecFromProductionPath(spec); err == nil && spec.Networks != nil {
		return spec, nil
	}

	// Fallback: load from dev default config dir
	cfgDir, files, err := common.GetDevDefaultConfigFiles("transceiver")
	if err != nil {
		logrus.WithField("component", "transceiver").Warnf("failed to get default config dir: %v", err)
		return defaultSpec(), nil
	}
	for _, f := range files {
		if f.Name() == "default_spec.yaml" {
			if err := utils.LoadFromYaml(cfgDir+"/"+f.Name(), spec); err == nil && spec.Networks != nil {
				return spec, nil
			}
		}
	}

	return defaultSpec(), nil
}

func defaultSpec() *TransceiverSpec {
	return &TransceiverSpec{
		Networks: map[string]*NetworkSpec{
			"business": {
				InterfacePatterns: []string{"mlx5_*", "ib*", "enp*s0f*", "bond*"},
				Thresholds: ThresholdSpec{
					TxPowerMarginDB: 1.0, RxPowerMarginDB: 1.0,
					TemperatureWarningC: 65, TemperatureCriticalC: 75,
				},
				CheckVendor: true, CheckLinkErrors: true,
				ApprovedVendors: []string{"Mellanox", "NVIDIA", "Innolight", "Hisense"},
			},
			"management": {
				InterfacePatterns: []string{"eno*", "eth0", "mgmt*"},
				Thresholds: ThresholdSpec{
					TxPowerMarginDB: 3.0, RxPowerMarginDB: 3.0,
					TemperatureWarningC: 75, TemperatureCriticalC: 85,
				},
				CheckVendor: false, CheckLinkErrors: false,
			},
		},
	}
}
```

- [ ] **Step 4: 创建 default_spec.yaml**

```yaml
# components/transceiver/config/default_spec.yaml
transceiver:
  networks:
    business:
      interface_patterns:
        - "mlx5_*"
        - "ib*"
        - "enp*s0f*"
        - "bond*"
      thresholds:
        tx_power_margin_db: 1.0
        rx_power_margin_db: 1.0
        temperature_warning_c: 65
        temperature_critical_c: 75
      check_vendor: true
      check_link_errors: true
      approved_vendors:
        - "Mellanox"
        - "NVIDIA"
        - "Innolight"
        - "Hisense"
    management:
      interface_patterns:
        - "eno*"
        - "eth0"
        - "mgmt*"
      thresholds:
        tx_power_margin_db: 3.0
        rx_power_margin_db: 3.0
        temperature_warning_c: 75
        temperature_critical_c: 85
      check_vendor: false
      check_link_errors: false
      approved_vendors: []
```

- [ ] **Step 5: 在 default_user_config.yaml 追加 transceiver 段**

在文件末尾追加：

```yaml
transceiver:
  query_interval: 30s
  cache_size: 5
  ignored_checkers: []
  enable_metrics: true
```

- [ ] **Step 6: 验证编译**

```bash
go build ./consts/... ./components/transceiver/config/...
```

Expected: 无错误

- [ ] **Step 7: Commit**

```bash
git add consts/consts.go components/transceiver/config/ config/default_user_config.yaml
git commit -m "feat(transceiver): add config, spec, and consts registration"
```

---

### Task 2: Collector — 数据结构 + 接口枚举 + ethtool 解析

**Files:**
- Create: `components/transceiver/collector/transceiver_info.go`
- Create: `components/transceiver/collector/utils.go`
- Create: `components/transceiver/collector/ethtool_dom.go`

- [ ] **Step 1: 创建 transceiver_info.go（Info 结构体 + Collect 入口）**

```go
// components/transceiver/collector/transceiver_info.go
package collector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

type TransceiverInfo struct {
	Modules []ModuleInfo `json:"modules"`
}

func (t *TransceiverInfo) JSON() (string, error) {
	data, err := json.MarshalIndent(t, "", "  ")
	return string(data), err
}

type ModuleInfo struct {
	Interface    string `json:"interface"`
	IBDev        string `json:"ib_dev"`
	NetworkType  string `json:"network_type"`
	CollectTool  string `json:"collect_tool"`

	Present      bool   `json:"present"`
	ModuleType   string `json:"module_type"`
	Vendor       string `json:"vendor"`
	PartNumber   string `json:"part_number"`
	SerialNumber string `json:"serial_number"`

	Temperature float64   `json:"temperature_c"`
	Voltage     float64   `json:"voltage_v"`
	TxPower     []float64 `json:"tx_power_dbm"`
	RxPower     []float64 `json:"rx_power_dbm"`
	BiasCurrent []float64 `json:"bias_current_ma"`

	TxPowerHighAlarm float64 `json:"tx_power_high_alarm_dbm"`
	TxPowerLowAlarm  float64 `json:"tx_power_low_alarm_dbm"`
	RxPowerHighAlarm float64 `json:"rx_power_high_alarm_dbm"`
	RxPowerLowAlarm  float64 `json:"rx_power_low_alarm_dbm"`
	TempHighAlarm    float64 `json:"temp_high_alarm_c"`
	TempLowAlarm     float64 `json:"temp_low_alarm_c"`
	VoltageHighAlarm float64 `json:"voltage_high_alarm_v"`
	VoltageLowAlarm  float64 `json:"voltage_low_alarm_v"`

	LinkErrors map[string]uint64 `json:"link_errors"`
}

type TransceiverCollector struct {
	networkClassifier *NetworkClassifier
}

func NewTransceiverCollector(classifier *NetworkClassifier) *TransceiverCollector {
	return &TransceiverCollector{networkClassifier: classifier}
}

func (c *TransceiverCollector) Name() string {
	return "TransceiverCollector"
}

func (c *TransceiverCollector) Collect(ctx context.Context) (*TransceiverInfo, error) {
	interfaces, err := EnumerateTransceiverInterfaces()
	if err != nil {
		return nil, fmt.Errorf("enumerate interfaces failed: %w", err)
	}

	info := &TransceiverInfo{}
	for _, iface := range interfaces {
		netType := c.networkClassifier.Classify(iface.Name)
		var module ModuleInfo

		if iface.IsIB {
			module, err = CollectMLXLink(iface.IBDev)
			module.Interface = iface.Name
			module.IBDev = iface.IBDev
			module.CollectTool = "mlxlink"
		} else {
			module, err = CollectEthtool(iface.Name)
			module.Interface = iface.Name
			module.CollectTool = "ethtool"
		}
		if err != nil {
			logrus.WithField("component", "transceiver").Warnf("collect %s failed: %v", iface.Name, err)
			module.Present = false
		}
		module.NetworkType = netType
		info.Modules = append(info.Modules, module)
	}

	return info, nil
}
```

- [ ] **Step 2: 创建 utils.go（接口枚举 + 网络分类）**

```go
// components/transceiver/collector/utils.go
package collector

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

type InterfaceEntry struct {
	Name string
	IsIB bool
	IBDev string
}

func EnumerateTransceiverInterfaces() ([]InterfaceEntry, error) {
	var entries []InterfaceEntry

	// Enumerate ethernet/RoCE interfaces from /sys/class/net/
	netDir := "/sys/class/net"
	netEntries, err := os.ReadDir(netDir)
	if err != nil {
		return nil, err
	}
	for _, e := range netEntries {
		name := e.Name()
		if name == "lo" || strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") {
			continue
		}
		// Check if this interface has a physical device (skip virtual)
		devicePath := filepath.Join(netDir, name, "device")
		if _, err := os.Stat(devicePath); os.IsNotExist(err) {
			continue
		}
		entries = append(entries, InterfaceEntry{Name: name, IsIB: false})
	}

	// Enumerate IB interfaces from /sys/class/infiniband/
	ibDir := "/sys/class/infiniband"
	ibEntries, err := os.ReadDir(ibDir)
	if err != nil {
		logrus.WithField("component", "transceiver").Debugf("no infiniband devices: %v", err)
		return entries, nil
	}
	for _, e := range ibEntries {
		ibDev := e.Name()
		// Skip virtual functions
		physfn := filepath.Join(ibDir, ibDev, "device", "physfn")
		if _, err := os.Stat(physfn); err == nil {
			continue
		}
		// Get corresponding net device
		netDevDir := filepath.Join(ibDir, ibDev, "device", "net")
		netDevs, _ := os.ReadDir(netDevDir)
		netDevName := ""
		if len(netDevs) > 0 {
			netDevName = netDevs[0].Name()
		}
		entries = append(entries, InterfaceEntry{Name: netDevName, IsIB: true, IBDev: ibDev})
	}

	return entries, nil
}

type NetworkClassifier struct {
	patterns map[string][]string // network_type -> patterns
}

func NewNetworkClassifier(patterns map[string][]string) *NetworkClassifier {
	return &NetworkClassifier{patterns: patterns}
}

func (c *NetworkClassifier) Classify(ifaceName string) string {
	// Check management first (usually more specific patterns)
	if patterns, ok := c.patterns["management"]; ok {
		for _, p := range patterns {
			if matched, _ := filepath.Match(p, ifaceName); matched {
				return "management"
			}
		}
	}
	// Check business
	if patterns, ok := c.patterns["business"]; ok {
		for _, p := range patterns {
			if matched, _ := filepath.Match(p, ifaceName); matched {
				return "business"
			}
		}
	}
	// Default to business (safer — stricter checks)
	return "business"
}
```

- [ ] **Step 3: 创建 ethtool_dom.go**

```go
// components/transceiver/collector/ethtool_dom.go
package collector

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
	"github.com/sirupsen/logrus"
)

func CollectEthtool(netdev string) (ModuleInfo, error) {
	module := ModuleInfo{Present: true}

	output, err := utils.ExecCommand(consts.CmdTimeout, "ethtool", "-m", netdev)
	if err != nil {
		return ModuleInfo{Present: false}, err
	}

	module.parseEthtoolModule(output)
	module.parseLinkErrors(netdev)
	return module, nil
}

func (m *ModuleInfo) parseEthtoolModule(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch {
		case key == "Identifier" || key == "Transceiver type":
			m.ModuleType = val
		case key == "Vendor name":
			m.Vendor = val
		case key == "Vendor PN":
			m.PartNumber = val
		case key == "Vendor SN":
			m.SerialNumber = val
		case key == "Module temperature":
			m.Temperature = parseFloat(val)
		case key == "Module voltage":
			m.Voltage = parseFloat(val)
		case strings.HasPrefix(key, "Laser tx power"):
			m.TxPower = append(m.TxPower, parseDBM(val))
		case strings.HasPrefix(key, "Receiver signal average optical power"):
			m.RxPower = append(m.RxPower, parseDBM(val))
		case strings.HasPrefix(key, "Laser tx bias current"):
			m.BiasCurrent = append(m.BiasCurrent, parseFloat(val))
		case key == "Module temperature high alarm threshold":
			m.TempHighAlarm = parseFloat(val)
		case key == "Module temperature low alarm threshold":
			m.TempLowAlarm = parseFloat(val)
		case key == "Laser tx power high alarm threshold":
			m.TxPowerHighAlarm = parseDBM(val)
		case key == "Laser tx power low alarm threshold":
			m.TxPowerLowAlarm = parseDBM(val)
		case key == "Laser rx power high alarm threshold":
			m.RxPowerHighAlarm = parseDBM(val)
		case key == "Laser rx power low alarm threshold":
			m.RxPowerLowAlarm = parseDBM(val)
		case key == "Module voltage high alarm threshold":
			m.VoltageHighAlarm = parseFloat(val)
		case key == "Module voltage low alarm threshold":
			m.VoltageLowAlarm = parseFloat(val)
		}
	}
}

func (m *ModuleInfo) parseLinkErrors(netdev string) {
	output, err := utils.ExecCommand(consts.CmdTimeout, "ethtool", "-S", netdev)
	if err != nil {
		logrus.WithField("component", "transceiver").Debugf("ethtool -S %s failed: %v", netdev, err)
		return
	}

	m.LinkErrors = make(map[string]uint64)
	errorKeys := []string{"rx_crc_errors", "rx_fcs_errors", "rx_errors", "tx_errors"}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		for _, key := range errorKeys {
			if strings.HasPrefix(line, key+":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					val, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
					m.LinkErrors[key] = val
				}
			}
		}
	}
}

var floatRegex = regexp.MustCompile(`[-+]?[0-9]*\.?[0-9]+`)

func parseFloat(s string) float64 {
	match := floatRegex.FindString(s)
	if match == "" {
		return 0
	}
	val, _ := strconv.ParseFloat(match, 64)
	return val
}

func parseDBM(s string) float64 {
	// ethtool outputs like "0.5012 mW / -3.00 dBm"
	if idx := strings.Index(s, "dBm"); idx > 0 {
		part := strings.TrimSpace(s[:idx])
		if slashIdx := strings.LastIndex(part, "/"); slashIdx > 0 {
			part = strings.TrimSpace(part[slashIdx+1:])
		}
		val, _ := strconv.ParseFloat(strings.TrimSpace(part), 64)
		return val
	}
	return parseFloat(s)
}
```

- [ ] **Step 4: 验证编译**

```bash
go build ./components/transceiver/...
```

Expected: 无错误

- [ ] **Step 5: Commit**

```bash
git add components/transceiver/collector/
git commit -m "feat(transceiver): add collector with ethtool DOM parsing and interface enumeration"
```

---

### Task 3: Collector — mlxlink 解析

**Files:**
- Create: `components/transceiver/collector/mlxlink_dom.go`

- [ ] **Step 1: 创建 mlxlink_dom.go**

```go
// components/transceiver/collector/mlxlink_dom.go
package collector

import (
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

func CollectMLXLink(ibDev string) (ModuleInfo, error) {
	module := ModuleInfo{Present: true}

	output, err := utils.ExecCommand(consts.CmdTimeout, "mlxlink", "-d", ibDev, "-m")
	if err != nil {
		if strings.Contains(err.Error(), "No cable") || strings.Contains(output, "No cable") {
			return ModuleInfo{Present: false}, nil
		}
		return ModuleInfo{Present: false}, err
	}

	module.parseMLXLink(output)
	module.parseIBCounters(ibDev)
	return module, nil
}

func (m *ModuleInfo) parseMLXLink(output string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		kv := strings.SplitN(line, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch {
		case key == "Cable Type" || key == "Identifier":
			m.ModuleType = val
		case key == "Vendor Name":
			m.Vendor = val
		case key == "Vendor Part Number":
			m.PartNumber = val
		case key == "Vendor Serial Number":
			m.SerialNumber = val
		case key == "Temperature":
			m.Temperature = parseFloat(val)
		case key == "Voltage":
			m.Voltage = parseFloat(val)
		case strings.HasPrefix(key, "Tx Power Lane"):
			m.TxPower = append(m.TxPower, parseDBM(val))
		case strings.HasPrefix(key, "Rx Power Lane"):
			m.RxPower = append(m.RxPower, parseDBM(val))
		case strings.HasPrefix(key, "Tx Bias Lane"):
			m.BiasCurrent = append(m.BiasCurrent, parseFloat(val))
		case key == "Temperature High Threshold":
			m.TempHighAlarm = parseFloat(val)
		case key == "Temperature Low Threshold":
			m.TempLowAlarm = parseFloat(val)
		}
	}
}

func (m *ModuleInfo) parseIBCounters(ibDev string) {
	m.LinkErrors = make(map[string]uint64)
	counterDir := "/sys/class/infiniband/" + ibDev + "/ports/1/counters"
	errorKeys := []string{"symbol_error_counter", "VL15_dropped", "link_error_recovery_counter", "link_downed_counter"}

	for _, key := range errorKeys {
		content, err := utils.ReadFileContent(counterDir + "/" + key)
		if err != nil {
			continue
		}
		val := parseUint64(strings.TrimSpace(content))
		m.LinkErrors[key] = val
	}
}

func parseUint64(s string) uint64 {
	match := floatRegex.FindString(s)
	if match == "" {
		return 0
	}
	val, err := strconv.ParseUint(match, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
```

注意：此文件需要在顶部 import 中加 `"strconv"`。

- [ ] **Step 2: 验证编译**

```bash
go build ./components/transceiver/...
```

- [ ] **Step 3: Commit**

```bash
git add components/transceiver/collector/mlxlink_dom.go
git commit -m "feat(transceiver): add mlxlink DOM data collector for IB transceivers"
```

---

### Task 4: Checkers — 光功率 + 温度 + 电压 + 偏置电流

**Files:**
- Create: `components/transceiver/checker/checker.go`
- Create: `components/transceiver/checker/check_tx_power.go`
- Create: `components/transceiver/checker/check_rx_power.go`
- Create: `components/transceiver/checker/check_temperature.go`
- Create: `components/transceiver/checker/check_voltage.go`
- Create: `components/transceiver/checker/check_bias_current.go`

- [ ] **Step 1: 创建 checker.go（注册入口）**

```go
// components/transceiver/checker/checker.go
package checker

import (
	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/config"
)

func NewCheckers(cfg *config.TransceiverUserConfig, spec *config.TransceiverSpec) ([]common.Checker, error) {
	checkers := []common.Checker{
		NewTxPowerChecker(spec),
		NewRxPowerChecker(spec),
		NewTemperatureChecker(spec),
		NewVoltageChecker(spec),
		NewBiasCurrentChecker(spec),
		NewVendorChecker(spec),
		NewLinkErrorsChecker(spec),
		NewPresenceChecker(spec),
	}

	ignoredMap := make(map[string]bool)
	if cfg != nil && cfg.Transceiver != nil {
		for _, v := range cfg.Transceiver.IgnoredCheckers {
			ignoredMap[v] = true
		}
	}

	var active []common.Checker
	for _, chk := range checkers {
		if !ignoredMap[chk.Name()] {
			active = append(active, chk)
		}
	}
	return active, nil
}
```

- [ ] **Step 2: 创建 check_tx_power.go**

```go
// components/transceiver/checker/check_tx_power.go
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type TxPowerChecker struct {
	spec *config.TransceiverSpec
}

func NewTxPowerChecker(spec *config.TransceiverSpec) *TxPowerChecker {
	return &TxPowerChecker{spec: spec}
}

func (c *TxPowerChecker) Name() string { return config.TxPowerCheckerName }

func (c *TxPowerChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for TxPowerChecker")
	}

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if !m.Present || len(m.TxPower) == 0 {
			continue
		}
		netSpec := getNetworkSpec(c.spec, m.NetworkType)
		if netSpec == nil {
			continue
		}
		margin := netSpec.Thresholds.TxPowerMarginDB

		for lane, power := range m.TxPower {
			lowThresh := m.TxPowerLowAlarm + margin
			highThresh := m.TxPowerHighAlarm - margin
			if highThresh == 0 && lowThresh == 0 {
				continue // no thresholds available
			}
			if power < lowThresh || (highThresh > 0 && power > highThresh) {
				status = consts.StatusAbnormal
				laneLevel := consts.LevelCritical
				if m.NetworkType == "management" {
					laneLevel = consts.LevelWarning
				}
				if consts.LevelPriority[laneLevel] > consts.LevelPriority[level] {
					level = laneLevel
				}
				details = append(details, fmt.Sprintf("%s lane%d tx_power=%.2fdBm (range: %.2f~%.2f)",
					m.Interface, lane, power, lowThresh, highThresh))
			}
		}
	}

	return &common.CheckerResult{
		Name:        config.TxPowerCheckerName,
		Description: "Check transceiver Tx optical power",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "TxPowerAbnormal",
	}, nil
}

func getNetworkSpec(spec *config.TransceiverSpec, netType string) *config.NetworkSpec {
	if spec == nil || spec.Networks == nil {
		return nil
	}
	if ns, ok := spec.Networks[netType]; ok {
		return ns
	}
	return nil
}
```

- [ ] **Step 3: 创建 check_rx_power.go**

```go
// components/transceiver/checker/check_rx_power.go
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type RxPowerChecker struct {
	spec *config.TransceiverSpec
}

func NewRxPowerChecker(spec *config.TransceiverSpec) *RxPowerChecker {
	return &RxPowerChecker{spec: spec}
}

func (c *RxPowerChecker) Name() string { return config.RxPowerCheckerName }

func (c *RxPowerChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for RxPowerChecker")
	}

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if !m.Present || len(m.RxPower) == 0 {
			continue
		}
		netSpec := getNetworkSpec(c.spec, m.NetworkType)
		if netSpec == nil {
			continue
		}
		margin := netSpec.Thresholds.RxPowerMarginDB

		for lane, power := range m.RxPower {
			lowThresh := m.RxPowerLowAlarm + margin
			highThresh := m.RxPowerHighAlarm - margin
			if highThresh == 0 && lowThresh == 0 {
				continue
			}
			if power < lowThresh || (highThresh > 0 && power > highThresh) {
				status = consts.StatusAbnormal
				laneLevel := consts.LevelCritical
				if m.NetworkType == "management" {
					laneLevel = consts.LevelWarning
				}
				if consts.LevelPriority[laneLevel] > consts.LevelPriority[level] {
					level = laneLevel
				}
				details = append(details, fmt.Sprintf("%s lane%d rx_power=%.2fdBm (range: %.2f~%.2f)",
					m.Interface, lane, power, lowThresh, highThresh))
			}
		}
	}

	return &common.CheckerResult{
		Name:        config.RxPowerCheckerName,
		Description: "Check transceiver Rx optical power",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "RxPowerAbnormal",
	}, nil
}
```

- [ ] **Step 4: 创建 check_temperature.go**

```go
// components/transceiver/checker/check_temperature.go
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type TemperatureChecker struct {
	spec *config.TransceiverSpec
}

func NewTemperatureChecker(spec *config.TransceiverSpec) *TemperatureChecker {
	return &TemperatureChecker{spec: spec}
}

func (c *TemperatureChecker) Name() string { return config.TemperatureCheckerName }

func (c *TemperatureChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for TemperatureChecker")
	}

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if !m.Present || m.Temperature == 0 {
			continue
		}
		netSpec := getNetworkSpec(c.spec, m.NetworkType)
		if netSpec == nil {
			continue
		}

		if m.Temperature >= netSpec.Thresholds.TemperatureCriticalC {
			status = consts.StatusAbnormal
			if consts.LevelPriority[consts.LevelCritical] > consts.LevelPriority[level] {
				level = consts.LevelCritical
			}
			details = append(details, fmt.Sprintf("%s temp=%.1fC (critical threshold: %.1fC)",
				m.Interface, m.Temperature, netSpec.Thresholds.TemperatureCriticalC))
		} else if m.Temperature >= netSpec.Thresholds.TemperatureWarningC {
			status = consts.StatusAbnormal
			if consts.LevelPriority[consts.LevelWarning] > consts.LevelPriority[level] {
				level = consts.LevelWarning
			}
			details = append(details, fmt.Sprintf("%s temp=%.1fC (warning threshold: %.1fC)",
				m.Interface, m.Temperature, netSpec.Thresholds.TemperatureWarningC))
		}
	}

	return &common.CheckerResult{
		Name:        config.TemperatureCheckerName,
		Description: "Check transceiver module temperature",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "TransceiverOverheat",
	}, nil
}
```

- [ ] **Step 5: 创建 check_voltage.go**

```go
// components/transceiver/checker/check_voltage.go
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type VoltageChecker struct {
	spec *config.TransceiverSpec
}

func NewVoltageChecker(spec *config.TransceiverSpec) *VoltageChecker {
	return &VoltageChecker{spec: spec}
}

func (c *VoltageChecker) Name() string { return config.VoltageCheckerName }

func (c *VoltageChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for VoltageChecker")
	}

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if !m.Present || m.Voltage == 0 {
			continue
		}
		if m.NetworkType == "management" {
			continue // skip management network
		}
		if m.VoltageLowAlarm == 0 && m.VoltageHighAlarm == 0 {
			continue // no thresholds
		}
		if m.Voltage < m.VoltageLowAlarm || (m.VoltageHighAlarm > 0 && m.Voltage > m.VoltageHighAlarm) {
			status = consts.StatusAbnormal
			if consts.LevelPriority[consts.LevelWarning] > consts.LevelPriority[level] {
				level = consts.LevelWarning
			}
			details = append(details, fmt.Sprintf("%s voltage=%.3fV (range: %.3f~%.3f)",
				m.Interface, m.Voltage, m.VoltageLowAlarm, m.VoltageHighAlarm))
		}
	}

	return &common.CheckerResult{
		Name:        config.VoltageCheckerName,
		Description: "Check transceiver supply voltage",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "VoltageAbnormal",
	}, nil
}
```

- [ ] **Step 6: 创建 check_bias_current.go**

```go
// components/transceiver/checker/check_bias_current.go
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type BiasCurrentChecker struct {
	spec *config.TransceiverSpec
}

func NewBiasCurrentChecker(spec *config.TransceiverSpec) *BiasCurrentChecker {
	return &BiasCurrentChecker{spec: spec}
}

func (c *BiasCurrentChecker) Name() string { return config.BiasCurrentCheckerName }

func (c *BiasCurrentChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for BiasCurrentChecker")
	}

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if !m.Present || len(m.BiasCurrent) == 0 {
			continue
		}
		if m.NetworkType == "management" {
			continue
		}
		for lane, bias := range m.BiasCurrent {
			if bias <= 0 {
				status = consts.StatusAbnormal
				if consts.LevelPriority[consts.LevelWarning] > consts.LevelPriority[level] {
					level = consts.LevelWarning
				}
				details = append(details, fmt.Sprintf("%s lane%d bias_current=%.2fmA (abnormal)",
					m.Interface, lane, bias))
			}
		}
	}

	return &common.CheckerResult{
		Name:        config.BiasCurrentCheckerName,
		Description: "Check transceiver laser bias current",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "BiasCurrentAbnormal",
	}, nil
}
```

- [ ] **Step 7: 验证编译**

```bash
go build ./components/transceiver/...
```

- [ ] **Step 8: Commit**

```bash
git add components/transceiver/checker/checker.go \
  components/transceiver/checker/check_tx_power.go \
  components/transceiver/checker/check_rx_power.go \
  components/transceiver/checker/check_temperature.go \
  components/transceiver/checker/check_voltage.go \
  components/transceiver/checker/check_bias_current.go
git commit -m "feat(transceiver): add power, temperature, voltage and bias current checkers"
```

---

### Task 5: Checkers — 厂商 + 链路错误 + 在位

**Files:**
- Create: `components/transceiver/checker/check_vendor.go`
- Create: `components/transceiver/checker/check_link_errors.go`
- Create: `components/transceiver/checker/check_presence.go`

- [ ] **Step 1: 创建 check_vendor.go**

```go
// components/transceiver/checker/check_vendor.go
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type VendorChecker struct {
	spec *config.TransceiverSpec
}

func NewVendorChecker(spec *config.TransceiverSpec) *VendorChecker {
	return &VendorChecker{spec: spec}
}

func (c *VendorChecker) Name() string { return config.VendorCheckerName }

func (c *VendorChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for VendorChecker")
	}

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if !m.Present || m.Vendor == "" {
			continue
		}
		netSpec := getNetworkSpec(c.spec, m.NetworkType)
		if netSpec == nil || !netSpec.CheckVendor || len(netSpec.ApprovedVendors) == 0 {
			continue
		}

		approved := false
		for _, v := range netSpec.ApprovedVendors {
			if strings.EqualFold(strings.TrimSpace(m.Vendor), strings.TrimSpace(v)) {
				approved = true
				break
			}
		}
		if !approved {
			status = consts.StatusAbnormal
			if consts.LevelPriority[consts.LevelWarning] > consts.LevelPriority[level] {
				level = consts.LevelWarning
			}
			details = append(details, fmt.Sprintf("%s vendor=%q not in approved list",
				m.Interface, m.Vendor))
		}
	}

	return &common.CheckerResult{
		Name:        config.VendorCheckerName,
		Description: "Check transceiver vendor approval",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "VendorNotApproved",
	}, nil
}
```

- [ ] **Step 2: 创建 check_link_errors.go**

```go
// components/transceiver/checker/check_link_errors.go
package checker

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type LinkErrorsChecker struct {
	spec       *config.TransceiverSpec
	mu         sync.Mutex
	prevErrors map[string]map[string]uint64 // interface -> counter_name -> value
}

func NewLinkErrorsChecker(spec *config.TransceiverSpec) *LinkErrorsChecker {
	return &LinkErrorsChecker{
		spec:       spec,
		prevErrors: make(map[string]map[string]uint64),
	}
}

func (c *LinkErrorsChecker) Name() string { return config.LinkErrorsCheckerName }

func (c *LinkErrorsChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for LinkErrorsChecker")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if !m.Present || m.LinkErrors == nil {
			continue
		}
		netSpec := getNetworkSpec(c.spec, m.NetworkType)
		if netSpec == nil || !netSpec.CheckLinkErrors {
			continue
		}

		prev, hasPrev := c.prevErrors[m.Interface]
		c.prevErrors[m.Interface] = m.LinkErrors

		if !hasPrev {
			continue // first collection, no delta
		}

		for key, curr := range m.LinkErrors {
			if prevVal, ok := prev[key]; ok && curr > prevVal {
				delta := curr - prevVal
				status = consts.StatusAbnormal
				if consts.LevelPriority[consts.LevelCritical] > consts.LevelPriority[level] {
					level = consts.LevelCritical
				}
				details = append(details, fmt.Sprintf("%s %s increased by %d (prev=%d, curr=%d)",
					m.Interface, key, delta, prevVal, curr))
			}
		}
	}

	return &common.CheckerResult{
		Name:        config.LinkErrorsCheckerName,
		Description: "Check transceiver link error counters",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "LinkErrorsIncreased",
	}, nil
}
```

- [ ] **Step 3: 创建 check_presence.go**

```go
// components/transceiver/checker/check_presence.go
package checker

import (
	"context"
	"fmt"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
)

type PresenceChecker struct {
	spec *config.TransceiverSpec
}

func NewPresenceChecker(spec *config.TransceiverSpec) *PresenceChecker {
	return &PresenceChecker{spec: spec}
}

func (c *PresenceChecker) Name() string { return config.PresenceCheckerName }

func (c *PresenceChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.TransceiverInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type for PresenceChecker")
	}

	var details []string
	status := consts.StatusNormal
	level := consts.LevelInfo

	for _, m := range info.Modules {
		if m.Present {
			continue
		}
		status = consts.StatusAbnormal
		modLevel := consts.LevelFatal
		if m.NetworkType == "management" {
			modLevel = consts.LevelWarning
		}
		if consts.LevelPriority[modLevel] > consts.LevelPriority[level] {
			level = modLevel
		}
		details = append(details, fmt.Sprintf("%s module not present", m.Interface))
	}

	return &common.CheckerResult{
		Name:        config.PresenceCheckerName,
		Description: "Check transceiver module presence",
		Status:      status,
		Level:       level,
		Detail:      strings.Join(details, "\n"),
		ErrorName:   "TransceiverMissing",
	}, nil
}
```

- [ ] **Step 4: 验证编译**

```bash
go build ./components/transceiver/...
```

- [ ] **Step 5: Commit**

```bash
git add components/transceiver/checker/check_vendor.go \
  components/transceiver/checker/check_link_errors.go \
  components/transceiver/checker/check_presence.go
git commit -m "feat(transceiver): add vendor, link errors, and presence checkers"
```

---

### Task 6: Component 主体 + Metrics

**Files:**
- Create: `components/transceiver/transceiver.go`
- Create: `components/transceiver/metrics/metrics.go`

- [ ] **Step 1: 创建 metrics.go**

```go
// components/transceiver/metrics/metrics.go
package metrics

import (
	"github.com/scitix/sichek/components/transceiver/collector"
	commonmetrics "github.com/scitix/sichek/metrics"
	"github.com/sirupsen/logrus"
)

type TransceiverMetrics struct{}

func NewTransceiverMetrics() *TransceiverMetrics {
	return &TransceiverMetrics{}
}

func (m *TransceiverMetrics) ExportMetrics(info *collector.TransceiverInfo) {
	if info == nil {
		return
	}
	for _, mod := range info.Modules {
		if !mod.Present {
			continue
		}
		labels := map[string]string{
			"interface":    mod.Interface,
			"network_type": mod.NetworkType,
			"vendor":       mod.Vendor,
			"module_type":  mod.ModuleType,
		}
		commonmetrics.SetGaugeMetric("transceiver_temperature_celsius", labels, mod.Temperature)
		commonmetrics.SetGaugeMetric("transceiver_voltage_volts", labels, mod.Voltage)
		for i, p := range mod.TxPower {
			laneLabels := copyLabels(labels)
			laneLabels["lane"] = string(rune('0' + i))
			commonmetrics.SetGaugeMetric("transceiver_tx_power_dbm", laneLabels, p)
		}
		for i, p := range mod.RxPower {
			laneLabels := copyLabels(labels)
			laneLabels["lane"] = string(rune('0' + i))
			commonmetrics.SetGaugeMetric("transceiver_rx_power_dbm", laneLabels, p)
		}
	}
	logrus.WithField("component", "transceiver").Debug("exported transceiver metrics")
}

func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
```

注意：此 step 可能需要根据 `metrics/` 包的实际 API 调整。如果 `metrics.SetGaugeMetric` 不存在，实现者需要检查 `metrics/prometheus.go` 中可用的函数并适配。

- [ ] **Step 2: 创建 transceiver.go（Component 主体）**

参照 `components/ethernet/ethernet.go` 的完整模式：

```go
// components/transceiver/transceiver.go
package transceiver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/transceiver/checker"
	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"

	"github.com/sirupsen/logrus"
)

type component struct {
	ctx           context.Context
	cancel        context.CancelFunc
	componentName string
	cfg           *config.TransceiverUserConfig
	cfgMutex      sync.Mutex
	collector     *collector.TransceiverCollector
	checkers      []common.Checker

	cacheMtx    sync.RWMutex
	cacheBuffer []*common.Result
	cacheInfo   []common.Info
	currIndex   int64
	cacheSize   int64

	service *common.CommonService
}

var (
	transceiverComponent     *component
	transceiverComponentOnce sync.Once
)

func NewComponent(cfgFile string, specFile string, ignoredCheckers []string) (common.Component, error) {
	var err error
	transceiverComponentOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred when creating transceiver component: %v", r)
			}
		}()
		transceiverComponent, err = newComponent(cfgFile, specFile, ignoredCheckers)
	})
	return transceiverComponent, err
}

func newComponent(cfgFile string, specFile string, ignoredCheckers []string) (*component, error) {
	ctx, cancel := context.WithCancel(context.Background())
	var err error
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	cfg := &config.TransceiverUserConfig{}
	err = common.LoadUserConfig(cfgFile, cfg)
	if err != nil || cfg.Transceiver == nil {
		logrus.WithField("component", "transceiver").Warnf("get user config failed, using defaults")
		cfg.Transceiver = &config.TransceiverConfig{
			QueryInterval: common.Duration{Duration: 30 * time.Second},
			CacheSize:     5,
		}
	}
	if len(ignoredCheckers) > 0 {
		cfg.Transceiver.IgnoredCheckers = ignoredCheckers
	}

	spec, err := config.LoadSpec(specFile)
	if err != nil {
		logrus.WithField("component", "transceiver").Warnf("failed to load spec: %v, using defaults", err)
		spec = nil
	}

	// Build network classifier from spec
	patterns := make(map[string][]string)
	if spec != nil && spec.Networks != nil {
		for netType, ns := range spec.Networks {
			patterns[netType] = ns.InterfacePatterns
		}
	}
	classifier := collector.NewNetworkClassifier(patterns)
	collectorInst := collector.NewTransceiverCollector(classifier)

	checkers, err := checker.NewCheckers(cfg, spec)
	if err != nil {
		return nil, err
	}

	cacheSize := cfg.Transceiver.CacheSize
	if cacheSize == 0 {
		cacheSize = 5
	}

	comp := &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameTransceiver,
		cfg:           cfg,
		collector:     collectorInst,
		checkers:      checkers,
		cacheBuffer:   make([]*common.Result, cacheSize),
		cacheInfo:     make([]common.Info, cacheSize),
		cacheSize:     cacheSize,
	}
	service := common.NewCommonService(ctx, cfg, comp.componentName, comp.GetTimeout(), comp.HealthCheck)
	comp.service = service

	return comp, nil
}

func (c *component) Name() string { return c.componentName }

func (c *component) HealthCheck(ctx context.Context) (*common.Result, error) {
	info, err := c.collector.Collect(ctx)
	if err != nil {
		logrus.WithField("component", "transceiver").Errorf("failed to collect info: %v", err)
		return nil, err
	}

	result := common.Check(ctx, c.componentName, info, c.checkers)

	c.cacheMtx.Lock()
	c.cacheBuffer[c.currIndex] = result
	c.cacheInfo[c.currIndex] = info
	c.currIndex = (c.currIndex + 1) % c.cacheSize
	c.cacheMtx.Unlock()

	if result.Status == consts.StatusAbnormal && consts.LevelPriority[result.Level] > consts.LevelPriority[consts.LevelInfo] {
		logrus.WithField("component", "transceiver").Errorf("Health Check Failed")
	} else {
		logrus.WithField("component", "transceiver").Infof("Health Check PASSED")
	}

	return result, nil
}

func (c *component) CacheResults() ([]*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheBuffer, nil
}

func (c *component) LastResult() (*common.Result, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	if c.currIndex == 0 {
		return c.cacheBuffer[c.cacheSize-1], nil
	}
	return c.cacheBuffer[c.currIndex-1], nil
}

func (c *component) CacheInfos() ([]common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	return c.cacheInfo, nil
}

func (c *component) LastInfo() (common.Info, error) {
	c.cacheMtx.RLock()
	defer c.cacheMtx.RUnlock()
	if c.currIndex == 0 {
		return c.cacheInfo[c.cacheSize-1], nil
	}
	return c.cacheInfo[c.currIndex-1], nil
}

func (c *component) Start() <-chan *common.Result   { return c.service.Start() }
func (c *component) Stop() error                    { return c.service.Stop() }
func (c *component) Status() bool                   { return c.service.Status() }
func (c *component) GetTimeout() time.Duration      { return c.cfg.GetQueryInterval().Duration }

func (c *component) Update(cfg common.ComponentUserConfig) error {
	c.cfgMutex.Lock()
	p, ok := cfg.(*config.TransceiverUserConfig)
	if !ok {
		c.cfgMutex.Unlock()
		return fmt.Errorf("wrong config type for transceiver")
	}
	c.cfg = p
	c.cfgMutex.Unlock()
	return c.service.Update(cfg)
}

func (c *component) PrintInfo(info common.Info, result *common.Result, summaryPrint bool) bool {
	allPassed := true
	utils.PrintTitle("Transceiver", "-")

	transceiverInfo, ok := info.(*collector.TransceiverInfo)
	if ok && transceiverInfo != nil {
		fmt.Printf("Modules detected: %d\n", len(transceiverInfo.Modules))
		for _, m := range transceiverInfo.Modules {
			if m.Present {
				fmt.Printf("  %s [%s] %s %s (%.1fC)\n", m.Interface, m.NetworkType, m.Vendor, m.PartNumber, m.Temperature)
			} else {
				fmt.Printf("  %s [%s] NOT PRESENT\n", m.Interface, m.NetworkType)
			}
		}
	}

	if result != nil {
		for _, r := range result.Checkers {
			if r.Status != consts.StatusNormal && r.Level != consts.LevelInfo {
				allPassed = false
				fmt.Printf("  %s%s%s: %s\n", consts.Red, r.Name, consts.Reset, strings.TrimRight(r.Detail, "\n"))
			}
		}
	}

	if allPassed {
		fmt.Printf("\n%sAll transceiver checks passed%s\n", consts.Green, consts.Reset)
	}
	fmt.Println()
	return allPassed
}
```

- [ ] **Step 3: 验证编译**

```bash
go build ./components/transceiver/...
```

如果 `metrics.SetGaugeMetric` 不存在，暂时注释掉 metrics 导出部分，后续再适配。

- [ ] **Step 4: Commit**

```bash
git add components/transceiver/transceiver.go components/transceiver/metrics/
git commit -m "feat(transceiver): add component main body and metrics export"
```

---

### Task 7: CLI 命令注册 + Daemon 集成

**Files:**
- Create: `cmd/command/component/transceiver.go`
- Modify: `cmd/command/command.go`
- Modify: `cmd/command/component/all.go`

- [ ] **Step 1: 创建 CLI 子命令**

```go
// cmd/command/component/transceiver.go
package component

import (
	"context"
	"strings"

	"github.com/scitix/sichek/cmd/command/spec"
	"github.com/scitix/sichek/components/transceiver"
	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewTransceiverCmd() *cobra.Command {
	var (
		cfgFile            string
		specFile           string
		ignoredCheckersStr string
		verbose            bool
	)
	cmd := &cobra.Command{
		Use:     "transceiver",
		Aliases: []string{"tr"},
		Short:   "Perform Transceiver (optical module) HealthCheck",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), consts.CmdTimeout)
			if !verbose {
				logrus.SetLevel(logrus.ErrorLevel)
				defer cancel()
			} else {
				logrus.SetLevel(logrus.DebugLevel)
				defer func() {
					logrus.WithField("component", "transceiver").Info("transceiver cmd context canceled")
					cancel()
				}()
			}

			resolvedCfgFile, err := spec.EnsureCfgFile(cfgFile)
			if err != nil {
				logrus.WithField("component", "transceiver").Errorf("failed to load cfgFile: %v", err)
			}
			resolvedSpecFile, err := spec.EnsureSpecFile(specFile)
			if err != nil {
				logrus.WithField("component", "transceiver").Errorf("failed to load specFile: %v", err)
			}

			var ignoredCheckers []string
			if len(ignoredCheckersStr) > 0 {
				ignoredCheckers = strings.Split(ignoredCheckersStr, ",")
			}

			component, err := transceiver.NewComponent(resolvedCfgFile, resolvedSpecFile, ignoredCheckers)
			if err != nil {
				logrus.WithField("component", "transceiver").Error(err)
				return
			}
			result, err := RunComponentCheck(ctx, component, consts.CmdTimeout)
			if err != nil {
				return
			}
			PrintCheckResults(true, result)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "cfg", "c", "", "Path to the user config file")
	cmd.Flags().StringVarP(&specFile, "spec", "s", "", "Path to the specification file")
	cmd.Flags().StringVarP(&ignoredCheckersStr, "ignored-checkers", "i", "", "Ignored checkers")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	return cmd
}
```

- [ ] **Step 2: 在 command.go 中注册子命令**

在 `cmd/command/command.go` 的 `NewRootCmd()` 函数中，找到 `rootCmd.AddCommand(component.NewSyslogCmd())` 附近，追加：

```go
rootCmd.AddCommand(component.NewTransceiverCmd())
```

- [ ] **Step 3: 在 all.go 的 NewComponent switch 中加 transceiver case**

在 `cmd/command/component/all.go` 的 `NewComponent` 函数中，在 `case consts.ComponentNameEthernet:` 之前或之后加：

```go
case consts.ComponentNameTransceiver:
	return transceiver.NewComponent(cfgFile, specFile, ignoredCheckers)
```

同时在文件顶部的 import 中加：

```go
"github.com/scitix/sichek/components/transceiver"
```

- [ ] **Step 4: 验证编译**

```bash
go build ./cmd/... ./components/transceiver/...
```

- [ ] **Step 5: 验证 CLI 帮助**

```bash
go run cmd/main.go --help
```

Expected: 输出中包含 `transceiver` 子命令

- [ ] **Step 6: Commit**

```bash
git add cmd/command/component/transceiver.go cmd/command/command.go cmd/command/component/all.go
git commit -m "feat(transceiver): register CLI command and daemon integration"
```

---

### Task 8: 全量编译验证 + 确认

**Files:** 无新改动

- [ ] **Step 1: 全量编译**

```bash
make clean && make
```

Expected: `build/bin/sichek` 生成成功

- [ ] **Step 2: 运行测试**

```bash
go test ./components/transceiver/... -v -count=1
```

Expected: 编译通过（可能没有测试文件，输出 `[no test files]`）

- [ ] **Step 3: 验证全量测试不受影响**

```bash
go test ./... -count=1 -timeout=5m 2>&1 | grep "^FAIL" || echo "ALL PASS"
```

Expected: ALL PASS

- [ ] **Step 4: 查看最终 git log**

```bash
git log --oneline main..HEAD
```

Expected: 包含所有 task 的 commits
