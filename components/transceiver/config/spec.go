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
	InterfacePatterns []string      `json:"interface_patterns" yaml:"interface_patterns"`
	MaxSpeedMbps      int           `json:"max_speed_mbps" yaml:"max_speed_mbps"` // 0 means no speed-based classification
	Thresholds        ThresholdSpec `json:"thresholds" yaml:"thresholds"`
	CheckVendor       bool          `json:"check_vendor" yaml:"check_vendor"`
	CheckLinkErrors   bool          `json:"check_link_errors" yaml:"check_link_errors"`
	ApprovedVendors   []string      `json:"approved_vendors" yaml:"approved_vendors"`
}

type ThresholdSpec struct {
	TxPowerMarginDB      float64 `json:"tx_power_margin_db" yaml:"tx_power_margin_db"`
	RxPowerMarginDB      float64 `json:"rx_power_margin_db" yaml:"rx_power_margin_db"`
	TemperatureWarningC  float64 `json:"temperature_warning_c" yaml:"temperature_warning_c"`
	TemperatureCriticalC float64 `json:"temperature_critical_c" yaml:"temperature_critical_c"`
}

func LoadSpec(file string) (*TransceiverSpec, error) {
	spec := &TransceiverSpec{}

	if file != "" {
		if err := utils.LoadFromYaml(file, spec); err == nil && spec.Networks != nil {
			return spec, nil
		}
	}

	if err := common.LoadSpecFromProductionPath(spec); err == nil && spec.Networks != nil {
		return spec, nil
	}

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
			"management": {
				MaxSpeedMbps: 100000, // <= 100G is management
				Thresholds: ThresholdSpec{
					TxPowerMarginDB: 3.0, RxPowerMarginDB: 3.0,
					TemperatureWarningC: 75, TemperatureCriticalC: 85,
				},
				CheckVendor: false, CheckLinkErrors: false,
			},
			"business": {
				Thresholds: ThresholdSpec{
					TxPowerMarginDB: 1.0, RxPowerMarginDB: 1.0,
					TemperatureWarningC: 65, TemperatureCriticalC: 75,
				},
				CheckVendor: true, CheckLinkErrors: true,
				ApprovedVendors: []string{"Mellanox", "NVIDIA", "Innolight", "Hisense"},
			},
		},
	}
}
