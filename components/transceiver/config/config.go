package config

import "github.com/scitix/sichek/components/common"

const (
	TxPowerCheckerName     = "check_tx_power"
	RxPowerCheckerName     = "check_rx_power"
	TemperatureCheckerName = "check_temperature"
	VoltageCheckerName     = "check_voltage"
	BiasCurrentCheckerName = "check_bias_current"
	VendorCheckerName      = "check_vendor"
	LinkErrorsCheckerName  = "check_link_errors"
	PresenceCheckerName    = "check_presence"
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
