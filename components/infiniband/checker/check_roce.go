package checker

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/infiniband/collector"
	"github.com/scitix/sichek/components/infiniband/config"
	"github.com/scitix/sichek/consts"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const connectivityCacheTTL = 30 * time.Second

type connectivityCacheEntry struct {
	isReachable  bool
	errorMessage string
	latency      time.Duration
	timestamp    time.Time
}

type RoCEChecker struct {
	name        string
	spec        *config.InfinibandSpec
	description string
	mu          sync.RWMutex
	connCache   map[string]*connectivityCacheEntry
}

func NewRoCEChecker(specCfg *config.InfinibandSpec) (common.Checker, error) {
	return &RoCEChecker{
		name:      config.CheckRoCE,
		spec:      specCfg,
		connCache: make(map[string]*connectivityCacheEntry),
	}, nil
}

func (c *RoCEChecker) Name() string {
	return c.name
}

func (c *RoCEChecker) Description() string {
	return c.description
}

func (c *RoCEChecker) GetSpec() common.CheckerSpec {
	return nil
}

const (
	icmpProbeTimeout = 2 * time.Second
	tcpProbeTimeout  = 1 * time.Second
)

func (c *RoCEChecker) _doConnectivityCheck(netDev, netGW string) (bool, time.Duration, string) {
	start := time.Now()
	// Option A: Try ICMP Ping (requires root permissions)
	// Note: The standard Go ICMP library does not directly support binding to a specific interface (`-I` option).
	// Connectivity tests are usually determined by the kernel's routing table for the egress interface. The netDev parameter here is mainly used for logging and logical grouping.
	err := c._icmpPing(netGW, icmpProbeTimeout)
	if err == nil {
		latency := time.Since(start)
		logrus.WithFields(logrus.Fields{"netdev": netDev, "gateway": netGW, "latency": latency}).Debug("ICMP ping successful.")
		return true, latency, ""
	}
	logrus.WithError(err).Warnf("ICMP ping to gateway %s failed, falling back to TCP check", netGW)

	// Option B: Fall back to TCP connection test (no special permissions required).
	// Short per-port timeout: a live gateway on the local segment answers fast;
	// routers usually have these ports closed, so this mainly bounds the tail
	// latency when ICMP is filtered.
	commonPorts := []string{"443", "80", "22"}
	for _, port := range commonPorts {
		conn, tcpErr := net.DialTimeout("tcp", net.JoinHostPort(netGW, port), tcpProbeTimeout)
		if tcpErr == nil {
			conn.Close()
			latency := time.Since(start)
			logrus.WithFields(logrus.Fields{"netdev": netDev, "gateway": netGW, "port": port, "latency": latency}).Debug("TCP dial successful.")
			return true, latency, ""
		}
	}

	// If all methods failed
	finalErrMsg := fmt.Sprintf("gateway '%s' is unreachable. All checks failed. Last error: %v", netGW, err)
	return false, time.Since(start), finalErrMsg
}

// _icmpPing sends an ICMP Echo request and waits for a matching Echo Reply.
// It validates the target is a real IPv4 address and that the reply comes from
// that target carrying our own ICMP ID — a raw ICMP socket otherwise receives
// every host's ICMP traffic, so an unvalidated ReadFrom returns success on
// unrelated packets (and "succeeds" even against a bogus/zero address).
func (c *RoCEChecker) _icmpPing(targetIP string, timeout time.Duration) error {
	ip := net.ParseIP(targetIP)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid IPv4 target %q", targetIP)
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return fmt.Errorf("icmp listen failed (check for root permissions): %w", err)
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  1,
			Data: []byte("RoCECheck"),
		},
	}
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return fmt.Errorf("failed to marshal icmp message: %w", err)
	}

	deadline := time.Now().Add(timeout)
	if _, err := conn.WriteTo(msgBytes, &net.IPAddr{IP: ip}); err != nil {
		return fmt.Errorf("icmp write failed: %w", err)
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	reply := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(reply)
		if err != nil {
			return err // read deadline exceeded or socket error
		}
		if peer.String() != ip.String() {
			continue // packet from a different host
		}
		// Protocol number 1 = ICMP for IPv4.
		rm, err := icmp.ParseMessage(1, reply[:n])
		if err != nil || rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		if echo, ok := rm.Body.(*icmp.Echo); ok && echo.ID == id {
			return nil
		}
	}
}

func (c *RoCEChecker) CheckGatewayReachable(netDev, netGW string) (bool, time.Duration, string) {
	cacheKey := netDev + ":" + netGW

	c.mu.RLock()
	entry, exists := c.connCache[cacheKey]
	if exists && time.Since(entry.timestamp) < connectivityCacheTTL {
		logrus.WithFields(logrus.Fields{"netdev": netDev, "gateway": netGW}).Debug("Connectivity cache hit.")
		c.mu.RUnlock()
		return entry.isReachable, entry.latency, entry.errorMessage
	}
	c.mu.RUnlock()

	// Run the (slow) probe WITHOUT holding the lock so concurrent probes of
	// different gateways actually run in parallel. A rare duplicate probe of the
	// same gateway is harmless (idempotent) and cheaper than serializing every
	// probe behind one mutex for up to several seconds each.
	isReachable, latency, errMsg := c._doConnectivityCheck(netDev, netGW)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.connCache[cacheKey] = &connectivityCacheEntry{
		isReachable:  isReachable,
		errorMessage: errMsg,
		latency:      latency,
		timestamp:    time.Now(),
	}

	return isReachable, latency, errMsg
}

func (c *RoCEChecker) checkRoCEVFSpec(IBDev string, info *collector.InfinibandInfo) (bool, string) {
	info.RLock()
	var vfSpec string
	ibNicRole := info.IBNicRole
	found := false
	for _, hwInfo := range info.IBHardWareInfo {
		if hwInfo.IBDev == IBDev {
			vfSpec = hwInfo.VFSpec
			found = true
			break
		}
	}
	info.RUnlock()

	if !found {
		return true, ""
	}

	if ibNicRole == "sriovNode" {
		if vfSpec != "127" {
			return false, fmt.Sprintf("RoCE vf spec is not 127, it is %s", vfSpec)
		}
	}
	logrus.WithField("component", "infiniband").Infof("RoCE vf spec is valid: %s for IBDev: %s", vfSpec, IBDev)
	return true, ""
}

func (c *RoCEChecker) checkRoCEVFNum(IBDev string, info *collector.InfinibandInfo) (bool, string) {
	info.RLock()
	var vfNum string
	ibNicRole := info.IBNicRole
	found := false
	for _, hwInfo := range info.IBHardWareInfo {
		if hwInfo.IBDev == IBDev {
			vfNum = hwInfo.VFNum
			found = true
			break
		}
	}
	info.RUnlock()

	if !found {
		return true, ""
	}

	if ibNicRole == "sriovNode" {
		if vfNum != "16" && vfNum != "32" {
			return false, fmt.Sprintf("RoCE vf number is not valid, it is %s", vfNum)
		}
	} else {
		if vfNum != "" && vfNum != "0" {
			return false, fmt.Sprintf("RoCE vf number is not 0 in non sriovNode, it is %s", vfNum)
		}
	}
	logrus.WithField("component", "infiniband").Infof("RoCE vf number is valid: %s for IBDev: %s", vfNum, IBDev)
	return true, ""
}

func (c *RoCEChecker) checkRoCEGWStatus(IBDev string, PFGW string) (bool, time.Duration, string) {
	isReachable, latency, errMsg := c.CheckGatewayReachable(IBDev, PFGW)
	if !isReachable {
		return false, latency, errMsg
	}
	logrus.WithFields(logrus.Fields{"component": "infiniband", "gateway": PFGW, "netdev": IBDev, "latency": latency}).Infof("Gateway %s is reachable for netdev %s (%s)", PFGW, IBDev, latency)

	return true, latency, ""
}

func (c *RoCEChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	if infinibandInfo == nil {
		return nil, fmt.Errorf("InfinibandInfo is nil")
	}

	type checkItemResult struct {
		item   string
		dev    string
		status bool
		info   string
	}
	// The following slices are placeholders for future use if you want to collect per-check results:
	var checkVFSpec []checkItemResult
	var checkVFNum []checkItemResult

	var checkPerVFSpec, checkPerVFNum checkItemResult
	checkPerVFSpec.item = "vfSpec"
	checkPerVFNum.item = "vfNum"

	// Perform checks
	type deviceInfo struct {
		IBDev     string
		NetDev    string
		PFGW      string
		LinkLayer string
	}
	var devices []deviceInfo
	infinibandInfo.RLock()
	// Dedupe per IBDev: VF/gateway checks are per-PCI-device, multi-plane
	// HCAs would otherwise fire 4× and pay the ICMP timeout 4×.
	seenDev := make(map[string]bool)
	for _, hw := range infinibandInfo.IBHardWareInfo {
		if seenDev[hw.IBDev] {
			continue
		}
		seenDev[hw.IBDev] = true
		devices = append(devices, deviceInfo{
			IBDev:     hw.IBDev,
			NetDev:    hw.NetDev,
			PFGW:      hw.PFGW,
			LinkLayer: hw.LinkLayer,
		})
	}
	infinibandInfo.RUnlock()

	ethernetCount := 0
	type gwEntry struct {
		dev, netdev, gw string
		skip            bool // IPv6-only or no IPv4 gateway → don't probe
	}
	var gwEntries []gwEntry
	for _, dev := range devices {
		if strings.Contains(dev.IBDev, "mlx_bond") {
			continue
		}
		// RoCE checks only apply to Ethernet-link-layer HCAs. Skip Infiniband
		// devices per-device so mixed nodes (IB + RoCE HCAs) still check their
		// RoCE NICs. EqualFold tolerates the sysfs "InfiniBand" spelling.
		if !strings.EqualFold(strings.TrimSpace(dev.LinkLayer), "Ethernet") {
			logrus.WithField("component", "infiniband").Infof("RoCE checks skipped for non-Ethernet device %s (link_layer=%s)", dev.IBDev, dev.LinkLayer)
			continue
		}
		ethernetCount++
		IBDev := dev.IBDev
		checkPerVFSpec.dev = IBDev
		checkPerVFNum.dev = IBDev

		// Check RoCE VF Spec
		VFSpecStatus, VFSpecInfo := c.checkRoCEVFSpec(IBDev, infinibandInfo)
		checkPerVFSpec.info = VFSpecInfo
		if VFSpecStatus {
			checkPerVFSpec.status = true
		} else {
			checkPerVFSpec.status = false
		}
		checkVFSpec = append(checkVFSpec, checkPerVFSpec)
		// Check RoCE VF Num
		VFNumStatus, VFNumInfo := c.checkRoCEVFNum(IBDev, infinibandInfo)
		checkPerVFNum.info = VFNumInfo
		if VFNumStatus {
			checkPerVFNum.status = true
		} else {
			checkPerVFNum.status = false
		}
		checkVFNum = append(checkVFNum, checkPerVFNum)

		// Classify the gateway now; probe in parallel after the loop. An empty
		// gateway (L2-only/no route) or the "IPV6" sentinel (IPv6-only interface)
		// means there is no IPv4 gateway to probe — not a failure, no network I/O.
		// (Probing an empty/invalid address otherwise falls through to the TCP
		// fallback and spuriously connects to a local port such as sshd on :22.)
		gwEntries = append(gwEntries, gwEntry{
			dev:    IBDev,
			netdev: dev.NetDev,
			gw:     dev.PFGW,
			skip:   dev.PFGW == "" || dev.PFGW == "IPV6",
		})
	}

	// Probe gateways concurrently: each probe can block up to a few seconds
	// (ICMP timeout + TCP fallback), so serial probing would make the whole
	// checker scale with the number of unreachable gateways and risk blowing the
	// component HealthCheck timeout. Bounded by the small per-node device count.
	type probeResult struct {
		reachable bool
		latency   time.Duration
		info      string
	}
	results := make([]probeResult, len(gwEntries))
	var wg sync.WaitGroup
	for i := range gwEntries {
		if gwEntries[i].skip {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, latency, info := c.checkRoCEGWStatus(gwEntries[i].dev, gwEntries[i].gw)
			results[i] = probeResult{reachable: ok, latency: latency, info: info}
		}(i)
	}
	wg.Wait()

	// Assemble per-device connectivity (reachable/unreachable/skipped + latency)
	// and embed it into the collector info so it flows into the snapshot.
	gwProbed := 0
	gwNotProbed := 0
	var unreachableGw []string
	var gwDetail string
	connectivity := make(map[string]*collector.RoCEGatewayStatus, len(gwEntries))
	for i, e := range gwEntries {
		st := &collector.RoCEGatewayStatus{IBDev: e.dev, NetDev: e.netdev, Gateway: e.gw}
		switch {
		case e.skip:
			st.State = "skipped"
			gwNotProbed++
		case results[i].reachable:
			st.State = "reachable"
			st.LatencyUs = results[i].latency.Microseconds()
			gwProbed++
		default:
			st.State = "unreachable"
			st.LatencyUs = results[i].latency.Microseconds()
			st.Error = results[i].info
			unreachableGw = append(unreachableGw, e.dev)
			gwDetail += results[i].info
		}
		connectivity[e.dev] = st
	}
	if len(connectivity) > 0 {
		infinibandInfo.Lock()
		infinibandInfo.RoCEConnectivity = connectivity
		infinibandInfo.Unlock()
	}

	if ethernetCount == 0 {
		logrus.WithField("checker", c.Name()).Info("No Ethernet/RoCE devices found, RoCE checks not applicable")
		result := config.InfinibandCheckItems[c.name]
		result.Status = consts.StatusNormal
		result.Detail = "RoCE checks are not applicable (no Ethernet/RoCE devices found)"
		return &result, nil
	}

	var detail string
	for index := range checkVFSpec {
		if !checkVFSpec[index].status {
			detail += fmt.Sprintf(checkVFSpec[index].info)
		}
	}

	for index := range checkVFNum {
		if !checkVFNum[index].status {
			detail += fmt.Sprintf(checkVFNum[index].info)
		}
	}

	detail += gwDetail

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal
	result.Detail = detail
	// Surface gateway-specific connectivity for the CLI summary line,
	// independent of the VF checks folded into this same result. Curr stays
	// empty on the no-Ethernet path above so the summary hides the row.
	switch {
	case len(unreachableGw) > 0:
		result.Curr = "Unreachable"
		result.Device = strings.Join(unreachableGw, ",")
	case gwProbed > 0:
		result.Curr = "Reachable"
	case gwNotProbed > 0:
		// Ethernet devices present but none had an IPv4 gateway to probe
		// (IPv6-only or L2-only): report N/A instead of a misleading "Reachable".
		result.Curr = "N/A"
	default:
		result.Curr = "Reachable"
	}

	if detail == "" {
		logrus.WithField("checker", c.Name()).Infof("Finish RoCE check, no error detail")
		result.Status = consts.StatusNormal
		result.Detail = "RoCE checks passed successfully"
		return &result, nil
	} else {
		logrus.WithField("checker", c.Name()).Errorf("RoCE checks failed with detail: %s", detail)
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("RoCE checks failed: %s", detail)
		result.ErrorName = "RoCENotEnabled"
		result.Suggestion = "review the RoCE check details and ensure RoCE is properly configured"
	}

	return &result, nil
}
