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
package checker

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/ethernet/collector"
	"github.com/scitix/sichek/components/ethernet/config"
	"github.com/scitix/sichek/consts"
)

func NewCheckers(cfg *config.EthernetUserConfig, spec *config.EthernetSpecConfig) ([]common.Checker, error) {
	checkers := []common.Checker{
		&L1Checker{
			spec:        spec,
			prevCRC:     make(map[string]int64),
			prevCarrier: make(map[string]int64),
			prevDrops:   make(map[string]int64),
		},
		&L2Checker{
			spec:             spec,
			prevLinkFailures: make(map[string]int64),
			prevActiveSlave:  make(map[string]string),
		},
		&L3Checker{spec: spec},
		&L4Checker{spec: spec},
		&L5Checker{spec: spec},
	}
	// Filter skipped checkers
	ignoredMap := make(map[string]bool)
	if cfg != nil && cfg.Ethernet != nil {
		for _, v := range cfg.Ethernet.IgnoredCheckers {
			ignoredMap[v] = true
		}
	}
	var activeCheckers []common.Checker
	for _, chk := range checkers {
		if !ignoredMap[chk.Name()] {
			activeCheckers = append(activeCheckers, chk)
		}
	}
	return activeCheckers, nil
}

type L1Checker struct {
	spec        *config.EthernetSpecConfig
	prevCRC     map[string]int64
	prevCarrier map[string]int64
	prevDrops   map[string]int64
}

func (c *L1Checker) Name() string { return config.EthernetL1CheckerName }

// extractInt parses an integer using regex from a string pattern
func extractInt(input, pattern string) int64 {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		val, _ := strconv.ParseInt(matches[1], 10, 64)
		return val
	}
	return 0
}

func (c *L1Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	expectedSpeed := "25000" // default to 25G
	if c.spec != nil && c.spec.Speed != "" {
		expectedSpeed = c.spec.Speed
	}

	for _, bond := range info.BondInterfaces {
		for slaveName, slaveState := range info.Slaves[bond] {
			if !slaveState.LinkDetected {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "LinkDown"
				result.Detail += fmt.Sprintf("物理网卡 %s 链路未检测到 UP。执行命令：ethtool %s，预期：Link detected: yes，当前发现未连接或 unknown。\n", slaveName, slaveName)
			}

			if len(info.SyslogErrors) > 0 {
				for _, errLine := range info.SyslogErrors {
					if strings.Contains(errLine, "tx timeout") && strings.Contains(errLine, slaveName) {
						result.Status = consts.StatusAbnormal
						result.Level = consts.LevelCritical
						result.ErrorName = "TxTimeout"
						result.Detail += fmt.Sprintf("网卡 %s 在内核日志发现 tx timeout。执行命令：dmesg | grep -iE 'eth|mlx|link'。\n", slaveName)
						break
					}
				}
			}

			// check speed
			speedStr := strconv.Itoa(slaveState.Speed)
			if speedStr != expectedSpeed && slaveState.Speed > 0 {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "SpeedMismatch"
				result.Detail += fmt.Sprintf("网卡 %s 速率不匹配。预期: %sMb/s，当前发现: %sMb/s。\n", slaveName, expectedSpeed, speedStr)
			}

			// Parse stats
			sStats := info.Stats[slaveName]

			// CRC errors
			currCRC := sStats.RXErrors // Approximation, standard ip -s link maps CRC errors to RX errors broadly. For exact CRC, ethtool parsing should remain, but for now we follow the general RX error growth.
			if prev, ok := c.prevCRC[slaveName]; ok && currCRC > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "CRCErrorsGrowing"
				result.Detail += fmt.Sprintf("网卡 %s RX (CRC) 错误持续增长。之前: %d，当前: %d。\n", slaveName, prev, currCRC)
			}
			c.prevCRC[slaveName] = currCRC

			// Carrier errors
			currCarrierIPS := sStats.Carrier
			if prev, ok := c.prevCarrier[slaveName]; ok && currCarrierIPS > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "CarrierErrorsGrowing"
				result.Detail += fmt.Sprintf("网卡 %s Carrier 错误持续增长。之前: %d，当前: %d。\n", slaveName, prev, currCarrierIPS)
			}
			c.prevCarrier[slaveName] = currCarrierIPS

			// Drops
			currDrops := sStats.Dropped
			if prev, ok := c.prevDrops[slaveName]; ok && currDrops > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "DropsGrowing"
				result.Detail += fmt.Sprintf("网卡 %s Drops 持续增长。之前: %d，当前: %d。\n", slaveName, prev, currDrops)
			}
			c.prevDrops[slaveName] = currDrops
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "请检查物理链路、网线、驱动版本(ethtool -i)，或查看 dmesg 中确认具体错误；如果是速率不匹配，请检查对应配置。"
	}

	return result, nil
}

type L2Checker struct {
	spec             *config.EthernetSpecConfig
	prevLinkFailures map[string]int64
	prevActiveSlave  map[string]string
}

func (c *L2Checker) Name() string { return config.EthernetL2CheckerName }
func (c *L2Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	expectedMinSlaves := 2
	if c.spec != nil && c.spec.MinSlaves > 0 {
		expectedMinSlaves = c.spec.MinSlaves
	}

	for _, bond := range info.BondInterfaces {
		bState, exists := info.Bonds[bond]
		if !exists || bState.Name == "" {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "BondingMissing"
			result.Detail = fmt.Sprintf("Bond %s 在 /proc/net/bonding 中缺失。\n", bond)
			continue
		}

		expectedMII := "up"
		procContent := info.ProcNetBonding[bond]
		if c.spec != nil && c.spec.MIIStatus != "" {
			expectedMII = c.spec.MIIStatus
		}

		if (expectedMII == "up" && !bState.IsUp) || !strings.Contains(procContent, "MII Status: "+expectedMII) {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "BondDown"
			result.Detail += fmt.Sprintf("Bond 接口 %s 的总体状态不符合预期。命令：cat /proc/net/bonding/%s，预期：MII Status: %s，当前不匹配(可能为 down)。\n", bond, bond, expectedMII)
		}

		// check MTU
		if c.spec != nil && c.spec.MTU != "" {
			expectedMTU, _ := strconv.Atoi(c.spec.MTU)
			if bState.MTU > 0 && bState.MTU != expectedMTU {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "MTUMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s 的 MTU 不匹配。预期: %d，实际: %d。\n", bond, expectedMTU, bState.MTU)
			}
		}

		// check xmit_hash_policy
		if c.spec != nil && c.spec.XmitHashPolicy != "" {
			if bState.XmitHashPolicy != "" && bState.XmitHashPolicy != c.spec.XmitHashPolicy {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "XmitHashPolicyMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s 的 xmit_hash_policy 不匹配。预期: %s，当前: %s。执行命令：cat /sys/class/net/%s/bonding/xmit_hash_policy。\n", bond, c.spec.XmitHashPolicy, bState.XmitHashPolicy, bond)
			}
		}

		// check slave count
		slaveCount := len(info.Slaves[bond])
		if slaveCount < expectedMinSlaves {
			if result.Status == consts.StatusNormal {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "SlaveCountMismatch"
			}
			result.Detail += fmt.Sprintf("Bond %s 的 slave 数量不足。预期至少: %d，实际: %d。\n", bond, expectedMinSlaves, slaveCount)
		}

		// check miimon, updelay, downdelay (fetching downdelay and updelay via regex since they aren't fully standard across systems on sysfs)
		miimon := int64(bState.Miimon)
		updelay := extractInt(procContent, `Up Delay \(ms\):\s*(\d+)`)
		downdelay := extractInt(procContent, `Down Delay \(ms\):\s*(\d+)`)

		expectedMiimon := int64(0)
		expectedUpDelay := int64(0)
		expectedDownDelay := int64(0)
		if c.spec != nil {
			if c.spec.Miimon > 0 {
				expectedMiimon = int64(c.spec.Miimon)
			}
			expectedUpDelay = int64(c.spec.UpDelay)
			expectedDownDelay = int64(c.spec.DownDelay)
		}

		if miimon == 0 {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "MiimonDisabled"
			result.Detail += fmt.Sprintf("Bond %s 的 MII Polling Interval (miimon) 为 0，未开启底层链路检测，这会导致物理链路断开时发生持续丢包。执行命令：cat /proc/net/bonding/%s，请务必开启！\n", bond, bond)
		} else {
			if expectedMiimon > 0 && miimon != expectedMiimon {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "MiimonMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s 的 miimon 不匹配。预期: %d ms，实际: %d ms。执行命令：cat /sys/class/net/%s/bonding/miimon。\n", bond, expectedMiimon, miimon, bond)
			}

			if downdelay < miimon {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "DowndelayTooSmall"
				result.Detail += fmt.Sprintf("Bond %s 的 downdelay (%d ms) 小于 miimon (%d ms)，配置不合理，可能导致不必要的震荡或丢包。执行命令：cat /proc/net/bonding/%s。\n", bond, downdelay, miimon, bond)
			} else if expectedDownDelay > 0 && downdelay != expectedDownDelay {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "DowndelayMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s 的 downdelay 不匹配。预期: %d ms，实际: %d ms。执行命令：cat /sys/class/net/%s/bonding/downdelay。\n", bond, expectedDownDelay, downdelay, bond)
			}

			if updelay == 0 {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "UpdelayZero"
				result.Detail += fmt.Sprintf("Bond %s 的 updelay 为 0，由于交换机端口转发协商需要时间，立即切回流量极易产生丢包黑洞，建议设置 updelay。执行命令：cat /proc/net/bonding/%s。\n", bond, bond)
			} else if expectedUpDelay > 0 && updelay != expectedUpDelay {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "UpdelayMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s 的 updelay 不匹配。预期: %d ms，实际: %d ms。执行命令：cat /sys/class/net/%s/bonding/updelay。\n", bond, expectedUpDelay, updelay, bond)
			}
		}

		// track active slave for flapping detection
		activeSlaveMatch := regexp.MustCompile(`Currently Active Slave:\s*(\w+)`).FindStringSubmatch(procContent)
		if len(activeSlaveMatch) > 1 {
			currActive := activeSlaveMatch[1]
			if prev, ok := c.prevActiveSlave[bond]; ok && prev != "" && prev != currActive {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "ActiveSlaveFlapping"
				result.Detail += fmt.Sprintf("Bond %s 发生了主备端口切换。之前主端口: %s，当前主端口: %s，如果频繁切换请重点关注物理层稳定性。\n", bond, prev, currActive)
			}
			c.prevActiveSlave[bond] = currActive
		}

		// track Link Failure Count per slave
		slavesData := strings.Split(procContent, "Slave Interface: ")
		for i := 1; i < len(slavesData); i++ {
			lines := strings.Split(slavesData[i], "\n")
			if len(lines) == 0 {
				continue
			}
			slaveName := strings.TrimSpace(lines[0])
			failCount := extractInt(slavesData[i], `Link Failure Count:\s*(\d+)`)
			trackKey := bond + "-" + slaveName

			if prev, ok := c.prevLinkFailures[trackKey]; ok && failCount > prev {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "LinkFailureGrowing"
				result.Detail += fmt.Sprintf("Bond %s 的从属网卡 %s 发生了链路断开(Link Failure)。之前故障次数: %d，当前: %d。\n", bond, slaveName, prev, failCount)
			}
			c.prevLinkFailures[trackKey] = failCount
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "请使用 cat /proc/net/bonding/bond0 核对 MII 状态及 Link Failure Count；确认配置文件(如 /etc/netplan 或 sysconfig) 中 miimon>0，且 slave 绑卡数量符合预期。"
	}

	return result, nil
}

type L3Checker struct{ spec *config.EthernetSpecConfig }

func (c *L3Checker) Name() string { return config.EthernetL3CheckerName }
func (c *L3Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	for _, bond := range info.BondInterfaces {
		bState, ok := info.Bonds[bond]
		if !ok || !strings.Contains(bState.Mode, "802.3ad") {
			continue
		}

		lacp, exists := info.LACP[bond]
		if !exists || lacp.PartnerMacAddress == "" {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "ActiveAggregatorMissing"
			result.Detail += fmt.Sprintf("Bond %s 配置为 802.3ad 模式，但未找到有效的 Active Aggregator 协商信息，可能对端交换机未配置 LACP 或链路异常。\n", bond)
			continue
		}

		if lacp.PartnerMacAddress == "00:00:00:00:00:00" {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelCritical
			result.ErrorName = "PartnerMacInvalid"
			result.Detail += fmt.Sprintf("Bond %s 的 Partner Mac Address 为全零。对端交换机未响应 LACP 报文，聚合失败。\n", bond)
		}

		for slaveName, sState := range info.Slaves[bond] {
			if !sState.IsUp {
				continue
			}

			if slaveAggID, ok := lacp.SlaveAggregatorIDs[slaveName]; ok && slaveAggID != lacp.ActiveAggregatorID {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelCritical
				result.ErrorName = "AggregatorMismatch"
				result.Detail += fmt.Sprintf("从属网卡 %s 的 Aggregator ID (%s) 与全局 Active Aggregator ID (%s) 不一致。该网卡虽然物理 UP，但在二层无法加入到数据转发聚合组中，请检查交换机端口配置或网线。\n", slaveName, slaveAggID, lacp.ActiveAggregatorID)
			}

			if portKey, ok := lacp.SlaveActorKeys[slaveName]; ok && portKey != lacp.ActorKey {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "ActorKeyMismatch"
				result.Detail += fmt.Sprintf("从属网卡 %s 的 port key (%s) 与全局 Actor Key (%s) 不一致。\n", slaveName, portKey, lacp.ActorKey)
			}

			if operKey, ok := lacp.SlavePartnerKeys[slaveName]; ok && operKey != lacp.PartnerKey {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "PartnerKeyMismatch"
				result.Detail += fmt.Sprintf("从属网卡 %s 的 oper key (%s) 与全局 Partner Key (%s) 不一致，对端交换机 LACP key 协商异常。\n", slaveName, operKey, lacp.PartnerKey)
			}
		}

		if c.spec != nil && c.spec.LACPRate != "" {
			if !strings.Contains(bState.LACPRate, c.spec.LACPRate) {
				if result.Status == consts.StatusNormal {
					result.Status = consts.StatusAbnormal
					result.Level = consts.LevelWarning
					result.ErrorName = "LACPRateMismatch"
				}
				result.Detail += fmt.Sprintf("Bond %s LACP rate 不匹配。命令：cat /sys/class/net/%s/bonding/lacp_rate，预期：%s，当前：%s。\n", bond, bond, c.spec.LACPRate, bState.LACPRate)
			}
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "建议同步排查对端交换机 (Switch) 上的 LACP / Eth-Trunk 聚合配置是否开启和匹配。"
	}

	return result, nil
}

type L4Checker struct{ spec *config.EthernetSpecConfig }

func (c *L4Checker) Name() string { return config.EthernetL4CheckerName }
func (c *L4Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	if strings.Contains(info.IPNeigh, "FAILED") || strings.Contains(info.IPNeigh, "INCOMPLETE") {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.ErrorName = "ARPFailed"
		result.Detail = "在 ARP 邻居表中发现 FAILED/INCOMPLETE 失败条目。这证明与某些邻居节点的二层 MAC 解析失败。"
		result.Suggestion = "请核对本端及交换机的 VLAN ID 配置及二层放行，或使用 arping 测试连通性并在 bond 口 tcpdump 抓弃 ARP request/reply。"
	}

	if info.Routes.GatewayIP != "" && !info.Routes.GatewayReachable {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelCritical
		result.ErrorName = "GatewayUnreachable"
		result.Detail += fmt.Sprintf("系统的网关 (%s) 不可达 (ping 失败，且不在 ARP 邻居表中)。\n", info.Routes.GatewayIP)
	}

	return result, nil
}

type L5Checker struct{ spec *config.EthernetSpecConfig }

func (c *L5Checker) Name() string { return config.EthernetL5CheckerName }
func (c *L5Checker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	info, ok := data.(*collector.EthernetInfo)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}

	result := &common.CheckerResult{
		Name:        c.Name(),
		Description: config.EthernetCheckItems[c.Name()],
		Status:      consts.StatusNormal,
		Level:       consts.LevelInfo,
		Curr:        "OK",
	}

	if !info.Routes.DefaultRouteViaBond {
		result.Status = consts.StatusAbnormal
		result.Level = consts.LevelWarning
		result.ErrorName = "DirectRouteMismatch"
		result.Detail += "系统的默认路由并非直接指向绑定的目标 bond 网卡，可能会导致预期业务流量不走 bond。\n"
	}

	if info.RPFilter["all"] == "1" {
		if result.Status == consts.StatusNormal {
			result.Status = consts.StatusAbnormal
			result.Level = consts.LevelWarning
			result.ErrorName = "RPFilterEnabled"
		}
		result.Detail += "系统启用了 rp_filter (all=1)，可能导致非对称路由丢包。命令：sysctl -n net.ipv4.conf.all.rp_filter，预期：0 或 2，当前：1。\n"
	}

	for bond, val := range info.RPFilter {
		if bond != "all" && val == "1" {
			if result.Status == consts.StatusNormal {
				result.Status = consts.StatusAbnormal
				result.Level = consts.LevelWarning
				result.ErrorName = "RPFilterEnabled"
			}
			result.Detail += fmt.Sprintf("Bond %s 启用了 rp_filter=1。预期：0 或 2。\n", bond)
		}
	}

	if result.Status != consts.StatusNormal {
		result.Suggestion = "如果发生丢包情况，建议检查路由匹配、多网卡下的策略路由(ip rule)，以及将 rp_filter 配置为 0(关闭) 或 2(松散模式)。"
	}

	return result, nil
}
