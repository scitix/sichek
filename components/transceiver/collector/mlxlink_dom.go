package collector

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

// CollectMLXLink collects transceiver DOM data via mlxlink.
// dev can be an IB device name (e.g. "mlx5_0") or a PCIe BDF (e.g. "0000:0e:00.0").
func CollectMLXLink(ctx context.Context, dev string) (ModuleInfo, error) {
	module := ModuleInfo{Present: true}

	cmdCtx, cancel := context.WithTimeout(ctx, consts.CmdTimeout)
	defer cancel()

	output, err := utils.ExecCommand(cmdCtx, "mlxlink", "-d", dev, "-m")
	if err != nil {
		outputStr := string(output)
		if strings.Contains(err.Error(), "No cable") || strings.Contains(outputStr, "No cable") ||
			strings.Contains(outputStr, "No module") {
			return ModuleInfo{Present: false}, nil
		}
		return ModuleInfo{Present: false}, err
	}

	module.parseMLXLink(string(output))
	return module, nil
}

// parseMLXLink parses mlxlink -m output.
// Actual format example:
//
//	Temperature [C]                    : 54 [-5..75]
//	Voltage [mV]                       : 3261.2 [2970..3630]
//	Bias Current [mA]                  : 8.380,8.380,8.380,8.380 [2..15]
//	Rx Power Current [dBm]             : 1.886,1.989,2.281,1.976 [-10.41..6]
//	Tx Power Current [dBm]             : 1.562,1.427,1.559,1.761 [-8.508..5]
//	Identifier                         : OSFP
//	Vendor Name                        : CLT
//	Vendor Part Number                 : T-F4GS-BV0
//	Vendor Serial Number               : CWJH05025502589
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
		case key == "Identifier":
			m.ModuleType = val
		case key == "Cable Type":
			if m.ModuleType == "" {
				m.ModuleType = val
			}
		case key == "Vendor Name":
			m.Vendor = val
		case key == "Vendor Part Number":
			m.PartNumber = val
		case key == "Vendor Serial Number":
			m.SerialNumber = val

		// Temperature [C] : 54 [-5..75]
		case strings.HasPrefix(key, "Temperature"):
			value, low, high := parseMLXLinkValueWithRange(val)
			m.Temperature = value
			m.TempLowAlarm = low
			m.TempHighAlarm = high

		// Voltage [mV] : 3261.2 [2970..3630]
		case strings.HasPrefix(key, "Voltage"):
			value, _, _ := parseMLXLinkValueWithRange(val)
			// mlxlink reports voltage in mV, convert to V
			m.Voltage = value / 1000.0

		// Bias Current [mA] : 8.380,8.380,8.380,8.380 [2..15]
		case strings.HasPrefix(key, "Bias Current"):
			m.BiasCurrent = parseMLXLinkMultiLane(val)

		// Rx Power Current [dBm] : 1.886,1.989,2.281,1.976 [-10.41..6]
		case strings.HasPrefix(key, "Rx Power Current"):
			values := parseMLXLinkMultiLane(val)
			m.RxPower = values
			_, low, high := parseMLXLinkValueWithRange(val)
			m.RxPowerLowAlarm = low
			m.RxPowerHighAlarm = high

		// Tx Power Current [dBm] : 1.562,1.427,1.559,1.761 [-8.508..5]
		case strings.HasPrefix(key, "Tx Power Current"):
			values := parseMLXLinkMultiLane(val)
			m.TxPower = values
			_, low, high := parseMLXLinkValueWithRange(val)
			m.TxPowerLowAlarm = low
			m.TxPowerHighAlarm = high
		}
	}
}

// parseMLXLinkValueWithRange parses "54 [-5..75]" → value=54, low=-5, high=75
func parseMLXLinkValueWithRange(s string) (value, low, high float64) {
	// Split on "[" to separate value from range
	parts := strings.SplitN(s, "[", 2)
	if len(parts) == 0 {
		return 0, 0, 0
	}

	// Parse the value part (may be comma-separated for multi-lane, take first)
	valStr := strings.TrimSpace(parts[0])
	if commaIdx := strings.Index(valStr, ","); commaIdx > 0 {
		valStr = valStr[:commaIdx]
	}
	value = parseFloat(valStr)

	// Parse the range [low..high]
	if len(parts) == 2 {
		rangeStr := strings.TrimRight(strings.TrimSpace(parts[1]), "]")
		rangeParts := strings.SplitN(rangeStr, "..", 2)
		if len(rangeParts) == 2 {
			low = parseFloat(rangeParts[0])
			high = parseFloat(rangeParts[1])
		}
	}

	return value, low, high
}

// parseMLXLinkMultiLane parses "8.380,8.380,8.380,8.380 [2..15]" → []float64{8.38, 8.38, 8.38, 8.38}
func parseMLXLinkMultiLane(s string) []float64 {
	// Remove range bracket part
	valPart := s
	if bracketIdx := strings.Index(s, "["); bracketIdx > 0 {
		valPart = strings.TrimSpace(s[:bracketIdx])
	}

	parts := strings.Split(valPart, ",")
	var values []float64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "N/A" {
			continue
		}
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			continue
		}
		values = append(values, v)
	}
	return values
}

func (m *ModuleInfo) parseIBCounters(ibDev string) {
	m.LinkErrors = make(map[string]uint64)
	counterDir := "/sys/class/infiniband/" + ibDev + "/ports/1/counters"
	errorKeys := []string{"symbol_error_counter", "VL15_dropped", "link_error_recovery_counter", "link_downed_counter"}

	for _, key := range errorKeys {
		data, err := os.ReadFile(counterDir + "/" + key)
		if err != nil {
			continue
		}
		valStr := strings.TrimSpace(string(data))
		val, _ := strconv.ParseUint(valStr, 10, 64)
		m.LinkErrors[key] = val
	}
}
