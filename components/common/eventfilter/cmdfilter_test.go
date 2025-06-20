package eventfilter

import (
	"encoding/json"
	"testing"

	"github.com/scitix/sichek/components/common"
	"github.com/stretchr/testify/assert"
)

func TestDmesgFilter(t *testing.T) {
	cmd := []string{"dmesg", "-T"}
	rules := map[string]*common.EventRuleConfig{
		"rule1": {
			Name:        "TestDemsg",
			LogFile:     "",
			Regexp:      "error",
			Level:       "warning",
			Description: "Test description",
			Suggestion:  "Test suggestion",
		},
	}
	cmdFilter, err := NewCommandFilter(cmd, rules, 100)
	if err != nil {
		t.Fatalf("failed to new cmd filter:%v", err)
	}
	defer cmdFilter.Close()
	result := cmdFilter.Check()
	js, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to convert result of EventFilter.Check() to json:%v", err)
	}
	print(string(js) + "\n")
	assert.Equal(t, 1, len(cmdFilter.Commands))
	assert.Equal(t, cmd[0], cmdFilter.Commands[0].Command)
	assert.Equal(t, cmd[1:], cmdFilter.Commands[0].Args)
	assert.Equal(t, "/tmp/sichek.dmesg.log", cmdFilter.LogFileName[0])
}
