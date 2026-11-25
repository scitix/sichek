package collector

import (
	"errors"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

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
	Interface   string `json:"interface"`
	IBDev       string `json:"ib_dev"`
	NetworkType string `json:"network_type"`
	CollectTool string `json:"collect_tool"`

	Present      bool   `json:"present"`
	ModuleType   string `json:"module_type"`
	// Recommendation is the mlxlink "Troubleshooting Info → Recommendation"
	// field (e.g. "Bad signal integrity"). It surfaces physical-layer signal
	// issues independently of error counters.
	Recommendation string `json:"recommendation"`
	Vendor       string `json:"vendor"`
	PartNumber   string `json:"part_number"`
	SerialNumber string `json:"serial_number"`
	LinkSpeed    string `json:"link_speed"`

	Temperature float64   `json:"temperature_c"`
	Voltage     float64   `json:"voltage_v"`
	TxPower     []float64 `json:"tx_power_dbm"`
	RxPower     []float64 `json:"rx_power_dbm"`
	BiasCurrent []float64 `json:"bias_current_ma"`

	// Per-lane module status flags, 1 = asserted: Rx/Tx loss of signal, Rx/Tx
	// CDR loss of lock and Tx fault. Collected and reported only; no checker
	// consumes them yet. Lanes follow the same split/trim as the power arrays,
	// so a dark lane of a wider module (which legitimately shows Rx LOS) is
	// dropped together with its -40 dBm reading.
	RxLOS   []int `json:"rx_los"`
	TxLOS   []int `json:"tx_los"`
	RxLOL   []int `json:"rx_lol"`
	TxLOL   []int `json:"tx_lol"`
	TxFault []int `json:"tx_fault"`

	TxPowerHighAlarm float64 `json:"tx_power_high_alarm_dbm"`
	TxPowerLowAlarm  float64 `json:"tx_power_low_alarm_dbm"`
	RxPowerHighAlarm float64 `json:"rx_power_high_alarm_dbm"`
	RxPowerLowAlarm  float64 `json:"rx_power_low_alarm_dbm"`
	TempHighAlarm    float64 `json:"temp_high_alarm_c"`
	TempLowAlarm     float64 `json:"temp_low_alarm_c"`
	VoltageHighAlarm float64 `json:"voltage_high_alarm_v"`
	VoltageLowAlarm  float64 `json:"voltage_low_alarm_v"`

	LinkErrors map[string]uint64 `json:"link_errors"`

	// Physical-layer counters and BER from `mlxlink -c` (IB / mlx5-BDF path only;
	// pure-ethtool interfaces leave these zero). All counters are cumulative since
	// TimeSinceLastClearMin, so downstream consumers must interpret them as deltas.
	// RawPhysicalBER is pre-FEC (a leading indicator); EffectivePhysicalBER/Errors
	// are post-FEC (non-zero means FEC can no longer mask the damage).
	RawPhysicalBER           float64 `json:"raw_physical_ber"`
	EffectivePhysicalBER     float64 `json:"effective_physical_ber"`
	EffectivePhysicalErrors  uint64  `json:"effective_physical_errors"`
	LinkDownCounter          uint64  `json:"link_down_counter"`
	LinkErrorRecoveryCounter uint64  `json:"link_error_recovery_counter"`
	TimeSinceLastClearMin    float64 `json:"time_since_last_clear_min"`
	// RawPhysicalErrors is the cumulative pre-FEC corrected-bit count
	// (mlx5 `rx_corrected_bits_phy`), populated on the ethtool path where the
	// RawPhysicalBER ratio is not available.
	RawPhysicalErrors uint64 `json:"raw_physical_errors"`

	// LinkLanes is the number of lanes the link is running on (`ethtool <if>`
	// "Lanes:"), 0 when unknown. Not exported as a metric.
	LinkLanes int `json:"-"`
}

type collectTask struct {
	iface InterfaceEntry
	// netdev is tried first with `ethtool -m` (~20ms). mlxDev, when set, is the
	// mlxlink fallback (~1-8s and holds the MFT device lock), used only when
	// there is no netdev or ethtool cannot decode the module.
	netdev string
	mlxDev string
}

type indexedModule struct {
	idx    int
	module ModuleInfo
	iface  InterfaceEntry
}

type TransceiverCollector struct {
	networkClassifier *NetworkClassifier

	// signalIntegrityInterval paces the mlxlink Recommendation sweep; <= 0
	// disables it. Recommendations are cached between sweeps so the
	// signal-integrity checker does not flap on rounds that skip mlxlink.
	signalIntegrityInterval time.Duration
	siMu                    sync.Mutex
	lastSISweep             time.Time
	recommendations         map[string]string // keyed by mlxlink device (BDF)
}

func NewTransceiverCollector(classifier *NetworkClassifier, signalIntegrityInterval time.Duration) *TransceiverCollector {
	return &TransceiverCollector{
		networkClassifier:       classifier,
		signalIntegrityInterval: signalIntegrityInterval,
		recommendations:         make(map[string]string),
	}
}

func (c *TransceiverCollector) Name() string {
	return "TransceiverCollector"
}

func (c *TransceiverCollector) Collect(ctx context.Context) (*TransceiverInfo, error) {
	interfaces, err := EnumerateTransceiverInterfaces()
	if err != nil {
		return nil, fmt.Errorf("enumerate interfaces failed: %w", err)
	}

	// Build the deduplicated task list: IB entries take priority over Ethernet entries
	// for the same interface name (mlxlink on the IB device gives richer data).
	// BDF-based deduplication prevents collecting VF-rep interfaces that share
	// the same physical PCIe function as their parent PF.
	seenName := make(map[string]bool)
	seenBDF := make(map[string]bool)
	var tasks []collectTask

	for _, iface := range interfaces {
		if !iface.IsIB {
			continue
		}
		if iface.Name != "" {
			seenName[iface.Name] = true
		}
		if iface.PcieBDF != "" {
			seenBDF[iface.PcieBDF] = true
		}
		tasks = append(tasks, collectTask{iface: iface, netdev: iface.Name, mlxDev: mlxlinkDev(iface)})
	}

	for _, iface := range interfaces {
		if iface.IsIB || seenName[iface.Name] {
			continue
		}
		if iface.PcieBDF != "" && seenBDF[iface.PcieBDF] {
			logrus.WithField("component", "transceiver").Debugf("skip %s: same BDF %s as IB interface", iface.Name, iface.PcieBDF)
			continue
		}
		task := collectTask{iface: iface, netdev: iface.Name}
		if iface.IsMLX5 && iface.PcieBDF != "" {
			task.mlxDev = iface.PcieBDF
		}
		tasks = append(tasks, task)
	}

	// Collect interfaces concurrently with a bounded worker pool. Only the
	// mlxlink fallback is expensive; it contends on the per-device MFT lock and
	// burns CPU, which is what used to starve the infiniband collector.
	const maxWorkers = 8
	resultCh := make(chan indexedModule, len(tasks))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(i int, task collectTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			module, ok := collectModule(ctx, task)
			if !ok {
				return
			}
			module.Interface = task.iface.Name
			if task.iface.IsIB {
				module.IBDev = task.iface.IBDev
			}

			resultCh <- indexedModule{idx: i, module: module, iface: task.iface}
		}(i, task)
	}

	wg.Wait()
	close(resultCh)

	// Preserve original order (IB before Ethernet, as tasks were ordered).
	ordered := make([]indexedModule, 0, len(tasks))
	for r := range resultCh {
		ordered = append(ordered, r)
	}
	sortByIndex(ordered)
	splitSharedModuleLanes(ordered)
	for i := range ordered {
		trimToLinkLanes(&ordered[i].module)
	}
	c.applySignalIntegrity(ctx, ordered)

	info := &TransceiverInfo{}
	for _, r := range ordered {
		module := r.module
		speedStr, speedMbps := GetLinkSpeed(r.iface.Name)
		module.LinkSpeed = speedStr
		module.NetworkType = c.networkClassifier.Classify(r.iface.Name, speedMbps)
		info.Modules = append(info.Modules, module)
	}

	return info, nil
}

// collectModule reads one interface: ethtool first, mlxlink only as a fallback.
// Collector entry points, overridable in tests.
var (
	collectEthtool = CollectEthtool
	collectMLXLink = CollectMLXLink
)

func collectModule(ctx context.Context, task collectTask) (ModuleInfo, bool) {
	if task.netdev != "" {
		module, err := collectEthtool(ctx, task.netdev)
		if err == nil {
			module.CollectTool = "ethtool"
			return module, true
		}
		logrus.WithField("component", "transceiver").Debugf("ethtool -m %s unusable: %v", task.netdev, err)
		// A module whose identity decoded but has no DOM page (e.g. some SFP28
		// AOCs on Intel ice NICs) has no mlxlink to fall back to. Report the
		// identity row as before rather than dropping the port.
		if task.mlxDev == "" && errors.Is(err, errNoDOM) {
			module.CollectTool = "ethtool"
			return module, true
		}
	}
	if task.mlxDev == "" {
		return ModuleInfo{}, false
	}
	module, err := collectMLXLink(ctx, task.mlxDev)
	if err != nil {
		logrus.WithField("component", "transceiver").Debugf("skip %s (mlxlink %s): %v", task.iface.Name, task.mlxDev, err)
		return ModuleInfo{}, false
	}
	module.CollectTool = "mlxlink"
	return module, true
}

// splitSharedModuleLanes handles multi-plane NICs where several PCIe functions
// share one physical cage. `ethtool -m` on any of them returns every lane of the
// module, so without splitting each plane would report the whole module and a
// single bad lane would be attributed to all planes. Plane function N owns lanes
// [N*k, N*k+k) where k = lanes / planes (verified on CX8 + OSFP 8x: 4 planes x 2
// lanes, matching mlxlink's per-plane view). Groups are keyed by module serial
// number; rows whose lane count does not divide evenly are left untouched.
func splitSharedModuleLanes(items []indexedModule) {
	groups := make(map[string][]int)
	for i, it := range items {
		if it.module.CollectTool != "ethtool" || it.module.SerialNumber == "" || it.iface.PcieBDF == "" {
			continue
		}
		groups[it.module.SerialNumber] = append(groups[it.module.SerialNumber], i)
	}
	for _, idxs := range groups {
		n := len(idxs)
		if n < 2 {
			continue
		}
		lanes := len(items[idxs[0]].module.RxPower)
		if lanes == 0 || lanes%n != 0 {
			continue
		}
		sort.Slice(idxs, func(a, b int) bool { return items[idxs[a]].iface.PcieBDF < items[idxs[b]].iface.PcieBDF })
		k := lanes / n
		for plane, idx := range idxs {
			m := &items[idx].module
			m.RxPower = laneSlice(m.RxPower, plane*k, k)
			m.TxPower = laneSlice(m.TxPower, plane*k, k)
			m.BiasCurrent = laneSlice(m.BiasCurrent, plane*k, k)
			for _, f := range m.laneFlags() {
				*f = laneSlice(*f, plane*k, k)
			}
		}
	}
}

// trimToLinkLanes drops module lanes beyond the link width. `ethtool -m`
// reports every lane of the module, including ones a narrower link leaves dark
// (e.g. a 4-lane 400G module carrying a 2-lane 200G link), which would otherwise
// trip the power and bias checkers. mlxlink only reports active lanes, so this
// restores parity. Active lanes are the first LinkLanes of the module, matching
// what mlxlink reports on the same port.
func trimToLinkLanes(m *ModuleInfo) {
	n := m.LinkLanes
	if n <= 0 {
		return
	}
	if len(m.RxPower) > n {
		m.RxPower = m.RxPower[:n]
	}
	if len(m.TxPower) > n {
		m.TxPower = m.TxPower[:n]
	}
	if len(m.BiasCurrent) > n {
		m.BiasCurrent = m.BiasCurrent[:n]
	}
	for _, f := range m.laneFlags() {
		if len(*f) > n {
			*f = (*f)[:n]
		}
	}
}

func laneSlice[T any](v []T, start, k int) []T {
	if start+k > len(v) {
		return v
	}
	out := make([]T, k)
	copy(out, v[start:start+k])
	return out
}

// applySignalIntegrity fills Recommendation for mlx5 modules. The ethtool path
// has no equivalent, so a plain `mlxlink -d <bdf>` (no -m/-c, ~0.5s) is run at
// most once per signalIntegrityInterval, one device at a time so it never holds
// more than one MFT lock or one CPU. Results are cached and reused on rounds in
// between. The sweep stops early when ctx is done; unswept devices keep their
// previous value and are retried next sweep.
func (c *TransceiverCollector) applySignalIntegrity(ctx context.Context, items []indexedModule) {
	c.siMu.Lock()
	defer c.siMu.Unlock()

	if c.signalIntegrityInterval > 0 && time.Since(c.lastSISweep) >= c.signalIntegrityInterval {
		c.lastSISweep = time.Now()
		swept := 0
		for _, it := range items {
			if it.module.CollectTool != "ethtool" || !it.iface.IsMLX5AndBDF() {
				continue
			}
			if ctx.Err() != nil {
				logrus.WithField("component", "transceiver").Warnf("signal-integrity sweep stopped after %d devices: %v", swept, ctx.Err())
				break
			}
			rec, err := CollectMLXLinkRecommendation(ctx, it.iface.PcieBDF)
			if err != nil {
				logrus.WithField("component", "transceiver").Debugf("mlxlink recommendation %s: %v", it.iface.PcieBDF, err)
				continue
			}
			c.recommendations[it.iface.PcieBDF] = rec
			swept++
		}
	}

	for i := range items {
		if items[i].module.CollectTool != "ethtool" {
			continue
		}
		if rec, ok := c.recommendations[items[i].iface.PcieBDF]; ok {
			items[i].module.Recommendation = rec
		}
	}
}

func sortByIndex(items []indexedModule) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].idx < items[j-1].idx; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
