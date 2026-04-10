package config

import (
	"testing"

	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
)

func TestGetCheckItem_Business(t *testing.T) {
	item := GetCheckItem(TxPowerCheckerName, "business")
	assert.Equal(t, TxPowerCheckerName, item.Name)
	assert.Equal(t, consts.LevelCritical, item.Level)
}

func TestGetCheckItem_Management(t *testing.T) {
	item := GetCheckItem(TxPowerCheckerName, "management")
	assert.Equal(t, TxPowerCheckerName, item.Name)
	assert.Equal(t, consts.LevelWarning, item.Level)
}

func TestGetCheckItem_PresenceBusiness(t *testing.T) {
	item := GetCheckItem(PresenceCheckerName, "business")
	assert.Equal(t, PresenceCheckerName, item.Name)
	assert.Equal(t, consts.LevelFatal, item.Level)
}

func TestGetCheckItem_PresenceManagement(t *testing.T) {
	item := GetCheckItem(PresenceCheckerName, "management")
	assert.Equal(t, PresenceCheckerName, item.Name)
	assert.Equal(t, consts.LevelWarning, item.Level)
}

func TestGetCheckItem_UnknownFallsBackToBusiness(t *testing.T) {
	// Unknown network type falls back to business items
	item := GetCheckItem(TxPowerCheckerName, "unknown_type")
	assert.Equal(t, TxPowerCheckerName, item.Name)
	assert.Equal(t, consts.LevelCritical, item.Level)
}

func TestGetCheckItem_UnknownCheckerName(t *testing.T) {
	// Completely unknown checker returns stub with just Name set
	item := GetCheckItem("unknown_checker", "business")
	assert.Equal(t, "unknown_checker", item.Name)
	assert.Equal(t, "", item.Level) // zero value
}

func TestGetCheckItem_AllBusinessLevels(t *testing.T) {
	expectations := map[string]string{
		TxPowerCheckerName:     consts.LevelCritical,
		RxPowerCheckerName:     consts.LevelCritical,
		TemperatureCheckerName: consts.LevelCritical,
		VoltageCheckerName:     consts.LevelCritical,
		BiasCurrentCheckerName: consts.LevelCritical,
		VendorCheckerName:      consts.LevelWarning,
		LinkErrorsCheckerName:  consts.LevelCritical,
		PresenceCheckerName:    consts.LevelFatal,
	}
	for name, expectedLevel := range expectations {
		t.Run(name, func(t *testing.T) {
			item := GetCheckItem(name, "business")
			assert.Equal(t, expectedLevel, item.Level, "checker: %s", name)
		})
	}
}

func TestGetCheckItem_AllManagementLevelsAreWarning(t *testing.T) {
	checkers := []string{
		TxPowerCheckerName,
		RxPowerCheckerName,
		TemperatureCheckerName,
		VoltageCheckerName,
		BiasCurrentCheckerName,
		VendorCheckerName,
		LinkErrorsCheckerName,
		PresenceCheckerName,
	}
	for _, name := range checkers {
		t.Run(name, func(t *testing.T) {
			item := GetCheckItem(name, "management")
			assert.Equal(t, consts.LevelWarning, item.Level, "checker: %s", name)
		})
	}
}
