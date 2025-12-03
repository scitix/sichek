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

func (c *RoCEChecker) _doConnectivityCheck(netDev, netGW string) (bool, string) {
	timeout := 2 * time.Second

	// Option A: Try ICMP Ping (requires root permissions)
	// Note: The standard Go ICMP library does not directly support binding to a specific interface (`-I` option).
	// Connectivity tests are usually determined by the kernel's routing table for the egress interface. The netDev parameter here is mainly used for logging and logical grouping.
	err := c._icmpPing(netGW, timeout)
	if err == nil {
		logrus.WithFields(logrus.Fields{"netdev": netDev, "gateway": netGW}).Debug("ICMP ping successful.")
		return true, ""
	}
	logrus.WithError(err).Warnf("ICMP ping to gateway %s failed, falling back to TCP check", netGW)

	// Option B: Fall back to TCP connection test (no special permissions required)
	commonPorts := []string{"443", "80", "22"}
	for _, port := range commonPorts {
		conn, tcpErr := net.DialTimeout("tcp", net.JoinHostPort(netGW, port), timeout)
		if tcpErr == nil {
			conn.Close()
			logrus.WithFields(logrus.Fields{"netdev": netDev, "gateway": netGW, "port": port}).Debug("TCP dial successful.")
			return true, ""
		}
	}

	// If all methods failed
	finalErrMsg := fmt.Sprintf("gateway '%s' is unreachable. All checks failed. Last error: %v", netGW, err)
	return false, finalErrMsg
}

// _icmpPing is a helper function that sends ICMP Echo requests
func (c *RoCEChecker) _icmpPing(targetIP string, timeout time.Duration) error {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return fmt.Errorf("icmp listen failed (check for root permissions): %w", err)
	}
	defer conn.Close()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("RoCECheck"),
		},
	}
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return fmt.Errorf("failed to marshal icmp message: %w", err)
	}

	if _, err := conn.WriteTo(msgBytes, &net.IPAddr{IP: net.ParseIP(targetIP)}); err != nil {
		return fmt.Errorf("icmp write failed: %w", err)
	}

	reply := make([]byte, 1500)
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	_, _, err = conn.ReadFrom(reply)
	return err
}

func (c *RoCEChecker) CheckGatewayReachable(netDev, netGW string) (bool, string) {
	cacheKey := netDev + ":" + netGW

	c.mu.RLock()
	entry, exists := c.connCache[cacheKey]
	if exists && time.Since(entry.timestamp) < connectivityCacheTTL {
		logrus.WithFields(logrus.Fields{"netdev": netDev, "gateway": netGW}).Debug("Connectivity cache hit.")
		c.mu.RUnlock()
		return entry.isReachable, entry.errorMessage
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists = c.connCache[cacheKey]
	if exists && time.Since(entry.timestamp) < connectivityCacheTTL {
		logrus.WithFields(logrus.Fields{"netdev": netDev, "gateway": netGW}).Debug("Connectivity cache hit after lock.")
		return entry.isReachable, entry.errorMessage
	}

	isReachable, errMsg := c._doConnectivityCheck(netDev, netGW)

	c.connCache[cacheKey] = &connectivityCacheEntry{
		isReachable:  isReachable,
		errorMessage: errMsg,
		timestamp:    time.Now(),
	}

	return isReachable, errMsg
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

func (c *RoCEChecker) checkRoCEGWStatus(IBDev string, PFGW string) (bool, string) {
	isReachable, errMsg := c.CheckGatewayReachable(IBDev, PFGW)
	if !isReachable {
		return false, errMsg
	}
	logrus.WithField("component", "infiniband").Infof("Gateway %s is reachable for netdev %s", PFGW, IBDev)

	return true, ""
}

func (c *RoCEChecker) Check(ctx context.Context, data any) (*common.CheckerResult, error) {
	infinibandInfo, ok := data.(*collector.InfinibandInfo)
	if !ok {
		return nil, fmt.Errorf("invalid InfinibandInfo type")
	}

	if infinibandInfo == nil {
		return nil, fmt.Errorf("InfinibandInfo is nil")
	}

	infinibandInfo.RLock()
	for _, hwInfo := range infinibandInfo.IBHardWareInfo {
		if hwInfo.LinkLayer == "Infiniband" {
			logrus.WithField("component", "infiniband").Infof("RoCE checks are not applicable for Infiniband devices: %s", hwInfo.IBDev)
			infinibandInfo.RUnlock()
			result := config.InfinibandCheckItems[c.name]
			result.Status = consts.StatusNormal
			result.Detail = "RoCE checks are not applicable for Infiniband devices"
			return &result, nil
		}
	}
	infinibandInfo.RUnlock()

	type checkItemResult struct {
		item   string
		dev    string
		status bool
		info   string
	}
	// The following slices are placeholders for future use if you want to collect per-check results:
	var checkVFSpec []checkItemResult
	var checkVFNum []checkItemResult
	var checkNetGw []checkItemResult

	var checkPerVFSpec, checkPerVFNum, checkPerNetGw checkItemResult
	checkPerVFSpec.item = "vfSpec"
	checkPerNetGw.item = "gwStatus"
	checkPerVFNum.item = "vfNum"

	// Perform checks
	type deviceInfo struct {
		IBDev string
		PFGW  string
	}
	var devices []deviceInfo
	infinibandInfo.RLock()
	for index := range infinibandInfo.IBHardWareInfo {
		devices = append(devices, deviceInfo{
			IBDev: infinibandInfo.IBHardWareInfo[index].IBDev,
			PFGW:  infinibandInfo.IBHardWareInfo[index].PFGW,
		})
	}
	infinibandInfo.RUnlock()

	for _, dev := range devices {
		IBDev := dev.IBDev
		checkPerVFSpec.dev = IBDev
		checkPerNetGw.dev = IBDev
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

		// Check RoCE Gateway Status
		NetGWStatus, NetGWInfo := c.checkRoCEGWStatus(IBDev, dev.PFGW)
		checkPerNetGw.info = NetGWInfo
		if NetGWStatus {
			checkPerNetGw.status = true
		} else if strings.Contains(NetGWInfo, "IPV6") {
			checkPerNetGw.status = true
		} else {
			checkPerNetGw.status = false
		}

		checkNetGw = append(checkNetGw, checkPerNetGw)
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

	for index := range checkNetGw {
		if !checkNetGw[index].status {
			detail += fmt.Sprintf(checkNetGw[index].info)
		}
	}

	result := config.InfinibandCheckItems[c.name]
	result.Status = consts.StatusNormal
	result.Detail = detail

	if detail == "" {
		result.Status = consts.StatusNormal
		result.Detail = "RoCE checks passed successfully"
		return &result, nil
	} else {
		result.Status = consts.StatusAbnormal
		result.Detail = fmt.Sprintf("RoCE checks failed: %s", detail)
		result.ErrorName = "RoCENotEnabled"
		result.Suggestion = "review the RoCE check details and ensure RoCE is properly configured"
	}

	return &result, nil
}
