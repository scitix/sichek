package collector

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/utils"
)

func CollectMLXLink(ctx context.Context, ibDev string) (ModuleInfo, error) {
	module := ModuleInfo{Present: true}

	cmdCtx, cancel := context.WithTimeout(ctx, consts.CmdTimeout)
	defer cancel()

	output, err := utils.ExecCommand(cmdCtx, "mlxlink", "-d", ibDev, "-m")
	if err != nil {
		outputStr := string(output)
		if strings.Contains(err.Error(), "No cable") || strings.Contains(outputStr, "No cable") {
			return ModuleInfo{Present: false}, nil
		}
		return ModuleInfo{Present: false}, err
	}

	module.parseMLXLink(string(output))
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
		data, err := os.ReadFile(counterDir + "/" + key)
		if err != nil {
			continue
		}
		valStr := strings.TrimSpace(string(data))
		val, _ := strconv.ParseUint(valStr, 10, 64)
		m.LinkErrors[key] = val
	}
}

