package checker

import (
	"context"
	"testing"

	"github.com/scitix/sichek/components/transceiver/collector"
	"github.com/scitix/sichek/components/transceiver/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSpec builds a TransceiverSpec suitable for unit tests.
// business: warn=65°C, crit=75°C, vendor check enabled with approved list.
// management: warn=75°C, crit=85°C, vendor check disabled.
func testSpec() *config.TransceiverSpec {
	return &config.TransceiverSpec{
		Networks: map[string]*config.NetworkSpec{
			"business": {
				Thresholds: config.ThresholdSpec{
					TemperatureWarningC:  65,
					TemperatureCriticalC: 75,
					TxPowerMarginDB:      1.0,
					RxPowerMarginDB:      1.0,
				},
				CheckVendor:     true,
				CheckLinkErrors: true,
				ApprovedVendors: []string{"Mellanox", "NVIDIA"},
			},
			"management": {
				Thresholds: config.ThresholdSpec{
					TemperatureWarningC:  75,
					TemperatureCriticalC: 85,
					TxPowerMarginDB:      3.0,
					RxPowerMarginDB:      3.0,
				},
				CheckVendor:     false,
				CheckLinkErrors: false,
			},
		},
	}
}

// ─── Temperature checker ──────────────────────────────────────────────────────

func TestTemperatureChecker_Normal(t *testing.T) {
	chk := &TemperatureChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: true, Temperature: 50},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, result.Status)
}

func TestTemperatureChecker_Warning(t *testing.T) {
	// 70°C >= warn(65), < crit(75) → StatusAbnormal, LevelWarning
	chk := &TemperatureChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: true, Temperature: 70},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, result.Status)
	assert.Equal(t, consts.LevelWarning, result.Level)
}

func TestTemperatureChecker_CriticalBusiness(t *testing.T) {
	// 80°C >= crit(75) on business → StatusAbnormal, level from check_item = LevelCritical
	chk := &TemperatureChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: true, Temperature: 80},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, result.Status)
	assert.Equal(t, consts.LevelCritical, result.Level)
}

func TestTemperatureChecker_CriticalManagement(t *testing.T) {
	// management: warn=75, crit=85; 80°C >= warn → StatusAbnormal, LevelWarning
	chk := &TemperatureChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "mgmt0", NetworkType: "management", Present: true, Temperature: 80},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, result.Status)
	// management check_item level = LevelWarning; warning branch uses LevelWarning directly
	assert.Equal(t, consts.LevelWarning, result.Level)
}

func TestTemperatureChecker_AbsentModuleSkipped(t *testing.T) {
	chk := &TemperatureChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: false, Temperature: 90},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	// Absent modules are skipped
	assert.Equal(t, consts.StatusNormal, result.Status)
}

// ─── Presence checker ─────────────────────────────────────────────────────────

func TestPresenceChecker_AllPresent(t *testing.T) {
	chk := &PresenceChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: true},
			{Interface: "eth1", NetworkType: "management", Present: true},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, result.Status)
}

func TestPresenceChecker_BusinessAbsent(t *testing.T) {
	chk := &PresenceChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: false},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, result.Status)
	// business presence level is LevelFatal
	assert.Equal(t, consts.LevelFatal, result.Level)
}

func TestPresenceChecker_ManagementAbsent(t *testing.T) {
	chk := &PresenceChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "mgmt0", NetworkType: "management", Present: false},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, result.Status)
	// management presence level is LevelWarning
	assert.Equal(t, consts.LevelWarning, result.Level)
}

func TestPresenceChecker_HighestLevelWins(t *testing.T) {
	// One management absent (warning) + one business absent (fatal) → fatal
	chk := &PresenceChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "mgmt0", NetworkType: "management", Present: false},
			{Interface: "eth0", NetworkType: "business", Present: false},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, result.Status)
	assert.Equal(t, consts.LevelFatal, result.Level)
}

// ─── Vendor checker ───────────────────────────────────────────────────────────

func TestVendorChecker_ApprovedVendor(t *testing.T) {
	chk := &VendorChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: true, Vendor: "Mellanox"},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, result.Status)
}

func TestVendorChecker_UnapprovedVendorBusiness(t *testing.T) {
	chk := &VendorChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: true, Vendor: "UnknownCo"},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusAbnormal, result.Status)
	// business vendor level = LevelWarning (from BusinessCheckItems)
	assert.Equal(t, consts.LevelWarning, result.Level)
}

func TestVendorChecker_ManagementSkipsVendorCheck(t *testing.T) {
	// management has check_vendor=false → any vendor passes
	chk := &VendorChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "mgmt0", NetworkType: "management", Present: true, Vendor: "RandomVendor"},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, result.Status)
}

func TestVendorChecker_CaseInsensitiveMatch(t *testing.T) {
	// "mellanox" (lowercase) should match "Mellanox" in approved list
	chk := &VendorChecker{spec: testSpec()}
	info := &collector.TransceiverInfo{
		Modules: []collector.ModuleInfo{
			{Interface: "eth0", NetworkType: "business", Present: true, Vendor: "mellanox"},
		},
	}

	result, err := chk.Check(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, consts.StatusNormal, result.Status)
}

// ─── Error handling ───────────────────────────────────────────────────────────

func TestCheckers_InvalidDataType(t *testing.T) {
	spec := testSpec()
	checkers := []interface {
		Check(context.Context, any) (*interface{ }, error)
	}{}
	_ = checkers

	// Quick check that each checker returns an error on wrong data type
	ctx := context.Background()
	wrongData := "not a TransceiverInfo"

	tempChk := &TemperatureChecker{spec: spec}
	_, err := tempChk.Check(ctx, wrongData)
	assert.Error(t, err)

	presChk := &PresenceChecker{spec: spec}
	_, err = presChk.Check(ctx, wrongData)
	assert.Error(t, err)

	vendChk := &VendorChecker{spec: spec}
	_, err = vendChk.Check(ctx, wrongData)
	assert.Error(t, err)
}
