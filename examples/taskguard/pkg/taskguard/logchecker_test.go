package taskguard

import (
	"testing"

	"github.com/scitix/taskguard/pkg/cfg"
	"github.com/stretchr/testify/assert"
)

func TestHasErrorInPodLogs(t *testing.T) {
	controller := &Controller{
		config: cfg.FaultToleranceConfig{
			LogCheckerRulesPath: "../../etc/log-checker-rules.yaml",
		},
	}

	tests := []struct {
		name     string
		logs     string
		expected bool
	}{
		{"NoError", "all good", false},
		{"OutOfMemoryError", "OutOfMemoryError happened", true},
		{"DistNetworkError", "DistNetworkError happened", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := controller.hasErrorInPodLogs(tt.logs)
			assert.Equal(t, tt.expected, result)
		})
	}
}
