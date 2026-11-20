package config

import (
	"time"

	"github.com/scitix/sichek/components/common"
)

const (
	TxPowerCheckerName     = "check_tx_power"
	RxPowerCheckerName     = "check_rx_power"
	TemperatureCheckerName = "check_temperature"
	VoltageCheckerName     = "check_voltage"
	BiasCurrentCheckerName = "check_bias_current"
	VendorCheckerName      = "check_vendor"
	LinkErrorsCheckerName  = "check_link_errors"
	PresenceCheckerName    = "check_presence"
	// SignalIntegrityCheckerName flags mlxlink "Bad signal integrity"
	// recommendations independently of error counters.
	SignalIntegrityCheckerName = "check_signal_integrity"
)

type TransceiverUserConfig struct {
	Transceiver *TransceiverConfig `json:"transceiver" yaml:"transceiver"`
}

type TransceiverConfig struct {
	QueryInterval   common.Duration `json:"query_interval" yaml:"query_interval"`
	CacheSize       int64           `json:"cache_size" yaml:"cache_size"`
	IgnoredCheckers []string        `json:"ignored_checkers" yaml:"ignored_checkers"`
	EnableMetrics   bool            `json:"enable_metrics" yaml:"enable_metrics"`
	// SignalIntegrityInterval paces the mlxlink Recommendation sweep that feeds
	// check_signal_integrity. Module DOM comes from ethtool every round; mlxlink
	// runs at most this often. Zero/unset means DefaultSignalIntegrityInterval;
	// ignore check_signal_integrity to turn the sweep off.
	SignalIntegrityInterval common.Duration `json:"signal_integrity_interval" yaml:"signal_integrity_interval"`
}

// DefaultSignalIntegrityInterval is used when signal_integrity_interval is unset.
const DefaultSignalIntegrityInterval = 30 * time.Minute

// GetSignalIntegrityInterval returns the sweep interval, or 0 when the
// signal-integrity checker is ignored (the sweep would be wasted work).
func (c *TransceiverConfig) GetSignalIntegrityInterval() time.Duration {
	for _, name := range c.IgnoredCheckers {
		if name == SignalIntegrityCheckerName {
			return 0
		}
	}
	if c.SignalIntegrityInterval.Duration > 0 {
		return c.SignalIntegrityInterval.Duration
	}
	return DefaultSignalIntegrityInterval
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
