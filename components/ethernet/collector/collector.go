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
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/scitix/sichek/pkg/utils"
)

type BondState struct {
	Name           string `json:"name"`
	IsUp           bool   `json:"is_up"`
	HasLowerUp     bool   `json:"has_lower_up"`
	IPAddr         string `json:"ip_addr"`
	MTU            int    `json:"mtu"`
	Mode           string `json:"mode"`
	Miimon         int    `json:"miimon"`
	XmitHashPolicy string `json:"xmit_hash_policy"`
	LACPRate       string `json:"lacp_rate"`
	ActiveSlave    string `json:"active_slave"`
}

type SlaveState struct {
	Name         string `json:"name"`
	IsUp         bool   `json:"is_up"`
	LinkDetected bool   `json:"link_detected"`
	Speed        int    `json:"speed"`  // Mbps
	Duplex       string `json:"duplex"` // "Full", "Half"
}

type LACPState struct {
	ActiveAggregatorID string            `json:"active_aggregator_id"`
	ActorKey           string            `json:"actor_key"`
	PartnerKey         string            `json:"partner_key"`
	PartnerMacAddress  string            `json:"partner_mac_address"`
	SlaveAggregatorIDs map[string]string `json:"slave_aggregator_ids"`
	SlaveActorKeys     map[string]string `json:"slave_actor_keys"`
	SlavePartnerKeys   map[string]string `json:"slave_partner_keys"`
}

type TrafficStats struct {
	RXErrors int64 `json:"rx_errors"`
	TXErrors int64 `json:"tx_errors"`
	Dropped  int64 `json:"dropped"`
	Carrier  int64 `json:"carrier"`
}

type RouteState struct {
	DefaultRouteViaBond bool   `json:"default_route_via_bond"`
	GatewayReachable    bool   `json:"gateway_reachable"`
	GatewayIP           string `json:"gateway_ip"`
}

type EthernetInfo struct {
	BondInterfaces []string
	Bonds          map[string]BondState
	Slaves         map[string]map[string]SlaveState // Bond -> SlaveName -> Info
	LACP           map[string]LACPState
	Stats          map[string]TrafficStats // Maps iface -> stats
	Routes         RouteState
	SyslogErrors   []string

	// Legacy string outputs (kept temporarily for backwards compatibility with un-migrated checkers)
	ProcNetBonding map[string]string
	BondSlaves     map[string][]string
	Ethtool        map[string]string
	EthtoolS       map[string]string
	EthtoolI       map[string]string
	IPSLink        map[string]string
	IPLink         string
	IPAddr         string
	IPRoute        string
	IPRule         string
	IPNeigh        string
	BridgeVlan     string
	BridgeFdb      string
	Dmesg          string
	RPFilter       map[string]string
	SysfsBonding   map[string]map[string]string
}

func (e *EthernetInfo) JSON() (string, error) {
	b, err := json.Marshal(e)
	return string(b), err
}

type EthernetCollector struct {
	name       string
	info       *EthernetInfo
	targetBond string
}

func NewEthernetCollector(targetBond string) (*EthernetCollector, error) {
	return &EthernetCollector{
		name:       "EthernetCollector",
		targetBond: targetBond,
		info: &EthernetInfo{
			Bonds:          make(map[string]BondState),
			Slaves:         make(map[string]map[string]SlaveState),
			LACP:           make(map[string]LACPState),
			Stats:          make(map[string]TrafficStats),
			SyslogErrors:   make([]string, 0),
			ProcNetBonding: make(map[string]string),
			SysfsBonding:   make(map[string]map[string]string),
			BondSlaves:     make(map[string][]string),
			Ethtool:        make(map[string]string),
			EthtoolS:       make(map[string]string),
			EthtoolI:       make(map[string]string),
			IPSLink:        make(map[string]string),
			RPFilter:       make(map[string]string),
		},
	}, nil
}

func (c *EthernetCollector) Name() string {
	return c.name
}

func (c *EthernetCollector) Collect(ctx context.Context) (*EthernetInfo, error) {
	out, _ := utils.ExecCommand(ctx, "ip", "-o", "link", "show", "type", "bond")
	lines := strings.Split(string(out), "\n")
	c.info.BondInterfaces = nil
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.Split(l, ": ")
		if len(parts) >= 2 {
			name := strings.Split(strings.TrimSpace(parts[1]), "@")[0]
			c.info.BondInterfaces = append(c.info.BondInterfaces, name)
		}
	}

	// Filter based on targetBond
	if c.targetBond != "" {
		var filtered []string
		for _, b := range c.info.BondInterfaces {
			if b == c.targetBond {
				filtered = append(filtered, b)
				break
			}
		}
		c.info.BondInterfaces = filtered
	}

	for _, bond := range c.info.BondInterfaces {
		c.info.Bonds[bond] = BondState{Name: bond}
		c.info.Slaves[bond] = make(map[string]SlaveState)

		outProc, _ := utils.ExecCommand(ctx, "cat", "/proc/net/bonding/"+bond)
		c.info.ProcNetBonding[bond] = string(outProc)

		// Parse BondState config from sysfs
		attrs := []string{"mode", "miimon", "slaves", "xmit_hash_policy", "lacp_rate"}
		if c.info.SysfsBonding[bond] == nil {
			c.info.SysfsBonding[bond] = make(map[string]string)
		}
		for _, attr := range attrs {
			outAttr, _ := utils.ExecCommand(ctx, "cat", "/sys/class/net/"+bond+"/bonding/"+attr)
			c.info.SysfsBonding[bond][attr] = strings.TrimSpace(string(outAttr))
		}

		slavesStr := c.info.SysfsBonding[bond]["slaves"]
		slaves := strings.Fields(slavesStr)
		c.info.BondSlaves[bond] = slaves

		// Fetch sysctl rp_filter for bond
		outRP, _ := utils.ExecCommand(ctx, "sysctl", "-n", "net.ipv4.conf."+bond+".rp_filter")
		c.info.RPFilter[bond] = strings.TrimSpace(string(outRP))

		// For each slave, fetch L1 info
		for _, slave := range slaves {
			outEth, _ := utils.ExecCommand(ctx, "ethtool", slave)
			c.info.Ethtool[slave] = string(outEth)

			outEthS, _ := utils.ExecCommand(ctx, "ethtool", "-S", slave)
			c.info.EthtoolS[slave] = string(outEthS)

			outEthI, _ := utils.ExecCommand(ctx, "ethtool", "-i", slave)
			c.info.EthtoolI[slave] = string(outEthI)

			outIPSL, _ := utils.ExecCommand(ctx, "ip", "-s", "link", "show", slave)
			c.info.IPSLink[slave] = string(outIPSL)
		}
	}

	var grepParts []string
	if c.targetBond != "" {
		grepParts = append(grepParts, c.targetBond)
	}

	filterArgs := []string{"grep", "-iE", strings.Join(grepParts, "|")}

	if len(grepParts) > 0 {
		outIPLink, _ := utils.ExecCommand(ctx, "sh", "-c", "ip -d link | "+strings.Join(filterArgs, " "))
		c.info.IPLink = string(outIPLink)

		outIPAddr, _ := utils.ExecCommand(ctx, "sh", "-c", "ip addr | "+strings.Join(filterArgs, " "))
		c.info.IPAddr = string(outIPAddr)

		outIPRoute, _ := utils.ExecCommand(ctx, "sh", "-c", "ip route | "+strings.Join(filterArgs, " "))
		c.info.IPRoute = string(outIPRoute)

		outIPRule, _ := utils.ExecCommand(ctx, "sh", "-c", "ip rule | "+strings.Join(filterArgs, " "))
		c.info.IPRule = string(outIPRule)

		outIPNeigh, _ := utils.ExecCommand(ctx, "sh", "-c", "ip neigh | "+strings.Join(filterArgs, " "))
		c.info.IPNeigh = string(outIPNeigh)

		outBridgeVlan, _ := utils.ExecCommand(ctx, "sh", "-c", "bridge vlan show | "+strings.Join(filterArgs, " "))
		c.info.BridgeVlan = string(outBridgeVlan)

		outBridgeFdb, _ := utils.ExecCommand(ctx, "sh", "-c", "bridge fdb show | "+strings.Join(filterArgs, " "))
		c.info.BridgeFdb = string(outBridgeFdb)
	} else {
		outIPLink, _ := utils.ExecCommand(ctx, "ip", "-d", "link")
		c.info.IPLink = string(outIPLink)

		outIPAddr, _ := utils.ExecCommand(ctx, "ip", "addr")
		c.info.IPAddr = string(outIPAddr)

		outIPRoute, _ := utils.ExecCommand(ctx, "ip", "route")
		c.info.IPRoute = string(outIPRoute)

		outIPRule, _ := utils.ExecCommand(ctx, "ip", "rule")
		c.info.IPRule = string(outIPRule)

		outIPNeigh, _ := utils.ExecCommand(ctx, "ip", "neigh")
		c.info.IPNeigh = string(outIPNeigh)

		outBridgeVlan, _ := utils.ExecCommand(ctx, "bridge", "vlan", "show")
		c.info.BridgeVlan = string(outBridgeVlan)

		outBridgeFdb, _ := utils.ExecCommand(ctx, "bridge", "fdb", "show")
		c.info.BridgeFdb = string(outBridgeFdb)
	}

	// Post-process parsed KV states for bonds
	for _, bond := range c.info.BondInterfaces {
		bState := c.info.Bonds[bond]
		bState.Mode = c.info.SysfsBonding[bond]["mode"]
		bState.XmitHashPolicy = c.info.SysfsBonding[bond]["xmit_hash_policy"]
		bState.LACPRate = c.info.SysfsBonding[bond]["lacp_rate"]
		bState.Miimon, _ = strconv.Atoi(c.info.SysfsBonding[bond]["miimon"])

		// Link states from IP Link / Addr
		bState.IsUp = strings.Contains(c.info.IPLink, fmt.Sprintf("%s: <BROADCAST,MULTICAST,MASTER,UP", bond))
		bState.HasLowerUp = strings.Contains(c.info.IPLink, fmt.Sprintf("%s: <BROADCAST,MULTICAST,MASTER,UP,LOWER_UP>", bond)) || strings.Contains(c.info.IPLink, "LOWER_UP")

		mtuMatch := regexp.MustCompile(fmt.Sprintf(`%s:.*mtu (\d+)`, bond)).FindStringSubmatch(c.info.IPLink)
		if len(mtuMatch) > 1 {
			bState.MTU, _ = strconv.Atoi(mtuMatch[1])
		}

		ipMatch := regexp.MustCompile(fmt.Sprintf(`inet ([\d\.]+)/\d+.*%s`, bond)).FindStringSubmatch(c.info.IPAddr)
		if len(ipMatch) > 1 {
			bState.IPAddr = ipMatch[1]
		}

		// Parse ProcNetBonding for Active Slave and 802.3ad info
		procStr := c.info.ProcNetBonding[bond]
		activeSlaveMatch := regexp.MustCompile(`Currently Active Slave:\s*(\w+)`).FindStringSubmatch(procStr)
		if len(activeSlaveMatch) > 1 {
			bState.ActiveSlave = activeSlaveMatch[1]
		}

		c.info.Bonds[bond] = bState

		// Populate LACP State
		if strings.Contains(procStr, "Bonding Mode: IEEE 802.3ad") {
			lacp := LACPState{
				SlaveAggregatorIDs: make(map[string]string),
				SlaveActorKeys:     make(map[string]string),
				SlavePartnerKeys:   make(map[string]string),
			}
			activeAggMatch := regexp.MustCompile(`(?s)Active Aggregator Info:\s*Aggregator ID:\s*(\d+).*?Actor Key:\s*(\d+).*?Partner Key:\s*(\d+).*?Partner Mac Address:\s*([\w:]+)`).FindStringSubmatch(procStr)
			if len(activeAggMatch) > 4 {
				lacp.ActiveAggregatorID = activeAggMatch[1]
				lacp.ActorKey = activeAggMatch[2]
				lacp.PartnerKey = activeAggMatch[3]
				lacp.PartnerMacAddress = activeAggMatch[4]
			}

			slavesData := strings.Split(procStr, "Slave Interface: ")
			for i := 1; i < len(slavesData); i++ {
				lines := strings.Split(slavesData[i], "\n")
				if len(lines) == 0 {
					continue
				}
				sName := strings.TrimSpace(lines[0])
				aggIDMatch := regexp.MustCompile(`Aggregator ID:\s*(\d+)`).FindStringSubmatch(slavesData[i])
				if len(aggIDMatch) > 1 {
					lacp.SlaveAggregatorIDs[sName] = aggIDMatch[1]
				}

				actorMatch := regexp.MustCompile(`port key:\s*(\d+)`).FindStringSubmatch(slavesData[i])
				if len(actorMatch) > 1 {
					lacp.SlaveActorKeys[sName] = actorMatch[1]
				}

				partnerMatch := regexp.MustCompile(`oper key:\s*(\d+)`).FindStringSubmatch(slavesData[i])
				if len(partnerMatch) > 1 {
					lacp.SlavePartnerKeys[sName] = partnerMatch[1]
				}
			}
			c.info.LACP[bond] = lacp
		}
	}

	for _, bond := range c.info.BondInterfaces {
		for _, slave := range c.info.BondSlaves[bond] {
			sState := SlaveState{Name: slave}

			// IsUp and Link detection
			outEth := c.info.Ethtool[slave]
			sState.LinkDetected = strings.Contains(outEth, "Link detected: yes")
			sState.IsUp = strings.Contains(c.info.IPLink, fmt.Sprintf("%s: <BROADCAST,MULTICAST,UP", slave)) || strings.Contains(c.info.IPLink, fmt.Sprintf("%s: <BROADCAST,MULTICAST,SLAVE,UP", slave))

			speedMatch := regexp.MustCompile(`Speed:\s*(\d+)Mb/s`).FindStringSubmatch(outEth)
			if len(speedMatch) > 1 {
				sState.Speed, _ = strconv.Atoi(speedMatch[1])
			}

			duplexMatch := regexp.MustCompile(`Duplex:\s*(\w+)`).FindStringSubmatch(outEth)
			if len(duplexMatch) > 1 {
				sState.Duplex = duplexMatch[1]
			}

			c.info.Slaves[bond][slave] = sState

			// Traffic Stats (ip -s link show)
			sStats := TrafficStats{}
			outIPSL := c.info.IPSLink[slave]
			lines := strings.Split(outIPSL, "\n")
			for i, line := range lines {
				if strings.Contains(line, "RX:") && i+1 < len(lines) {
					fields := strings.Fields(lines[i+1])
					if len(fields) >= 4 {
						sStats.RXErrors, _ = strconv.ParseInt(fields[2], 10, 64)
						sStats.Dropped, _ = strconv.ParseInt(fields[3], 10, 64)
					}
				}
				if strings.Contains(line, "TX:") && i+1 < len(lines) {
					fields := strings.Fields(lines[i+1])
					if len(fields) >= 4 {
						sStats.TXErrors, _ = strconv.ParseInt(fields[2], 10, 64)
						sStats.Carrier, _ = strconv.ParseInt(fields[3], 10, 64)
					}
				}
			}

			c.info.Stats[slave] = sStats
		}
	}

	outRPAll, _ := utils.ExecCommand(ctx, "sysctl", "-n", "net.ipv4.conf.all.rp_filter")
	c.info.RPFilter["all"] = strings.TrimSpace(string(outRPAll))

	// Parse RouteState
	rState := RouteState{}
	routeLines := strings.Split(c.info.IPRoute, "\n")
	for _, line := range routeLines {
		if strings.HasPrefix(line, "default via ") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				rState.GatewayIP = fields[2]
				if fields[4] == c.targetBond || (c.targetBond == "" && len(c.info.BondInterfaces) > 0 && fields[4] == c.info.BondInterfaces[0]) {
					rState.DefaultRouteViaBond = true
				}
			}
			break
		}
	}

	if rState.GatewayIP != "" {
		// Check reachable in neigh
		neighMatch := regexp.MustCompile(fmt.Sprintf(`%s\s+dev\s+.*?lladdr.*?REACHABLE`, regexp.QuoteMeta(rState.GatewayIP))).FindStringSubmatch(c.info.IPNeigh)
		if len(neighMatch) > 0 {
			rState.GatewayReachable = true
		} else {
			// Fallback ping if not in neigh Cache instantly
			pingOut, _ := utils.ExecCommand(ctx, "ping", "-c", "1", "-W", "1", rState.GatewayIP)
			if strings.Contains(string(pingOut), "1 received") {
				rState.GatewayReachable = true
			}
		}
	}
	c.info.Routes = rState

	// Parse Syslog Errors
	dmesgGrep := "eth|mlx|link|bond"
	if len(grepParts) > 0 {
		dmesgGrep = strings.Join(grepParts, "|")
	}
	outDmesg, _ := utils.ExecCommand(ctx, "sh", "-c", fmt.Sprintf("dmesg | grep -iE '%s' | tail -n 100", dmesgGrep))

	dmesgStr := string(outDmesg)
	c.info.Dmesg = dmesgStr

	for _, l := range strings.Split(dmesgStr, "\n") {
		lowerLine := strings.ToLower(l)
		if strings.Contains(lowerLine, "down") || strings.Contains(lowerLine, "fail") || strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "flap") {
			c.info.SyslogErrors = append(c.info.SyslogErrors, strings.TrimSpace(l))
		}
	}

	// Also append journalctl errors specifically for bonding
	outJournal, err := utils.ExecCommand(ctx, "sh", "-c", fmt.Sprintf("journalctl -k -S \"1 hour ago\" | grep -iE '%s' | grep -iE 'down|fail|flap|error' | tail -n 20", dmesgGrep))
	if err == nil {
		for _, l := range strings.Split(string(outJournal), "\n") {
			if strings.TrimSpace(l) != "" {
				c.info.SyslogErrors = append(c.info.SyslogErrors, strings.TrimSpace(l))
			}
		}
	}

	return c.info, nil
}
