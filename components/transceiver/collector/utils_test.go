package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsMezzanine(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Netdev and IB spellings seen in the field: "mezz0"/"mezz_0" on a
		// multi-plane B300 host, "mezz_0" for both netdev and IB device on thg1.
		{name: "mezz0", want: true},
		{name: "mezz3", want: true},
		{name: "mezz_0", want: true},
		{name: "mezz_3", want: true},
		// Substring match, matching how the other packages exclude mezzanine cards.
		{name: "bond_mezz0", want: true},
		// Real transceiver-bearing interfaces must not be caught.
		{name: "eth_r0_p0", want: false},
		{name: "roce_r0", want: false},
		{name: "mlx5_0", want: false},
		{name: "ib0", want: false},
		{name: "mgmt0", want: false},
		{name: "eth0", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isMezzanine(tt.name))
		})
	}
}

func newTestClassifier() *NetworkClassifier {
	return NewNetworkClassifier(
		map[string][]string{
			"management": {"eth*", "mgmt*"},
			"business":   {"rdma*"},
		},
		100000, // <= 100000 Mbps is management
	)
}

func TestNetworkClassifier_SpeedBased(t *testing.T) {
	c := newTestClassifier()

	// 400 Gbps > 100 Gbps → business
	assert.Equal(t, "business", c.Classify("rdma0", 400000))

	// 25 Gbps <= 100 Gbps → management
	assert.Equal(t, "management", c.Classify("rdma0", 25000))

	// Exactly 100 Gbps (boundary) → management
	assert.Equal(t, "management", c.Classify("rdma0", 100000))
}

func TestNetworkClassifier_PatternFallback(t *testing.T) {
	c := newTestClassifier()

	// Speed 0 → fall through to pattern matching; "eth0" matches "eth*" → management
	assert.Equal(t, "management", c.Classify("eth0", 0))

	// Speed 0, "mgmt0" matches "mgmt*" → management
	assert.Equal(t, "management", c.Classify("mgmt0", 0))

	// Speed 0, "rdma0" matches "rdma*" → business
	assert.Equal(t, "business", c.Classify("rdma0", 0))
}

func TestNetworkClassifier_DefaultBusiness(t *testing.T) {
	c := newTestClassifier()

	// No speed match, no pattern match → default "business"
	assert.Equal(t, "business", c.Classify("unknownif0", 0))
}

func TestNetworkClassifier_NoManagementMaxSpeed(t *testing.T) {
	// managementMaxMbps = 0 disables speed-based classification
	c := NewNetworkClassifier(map[string][]string{}, 0)

	// Should not classify as management even at low speed
	assert.Equal(t, "business", c.Classify("eth0", 25000))
}
