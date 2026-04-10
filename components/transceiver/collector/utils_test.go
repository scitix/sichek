package collector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
