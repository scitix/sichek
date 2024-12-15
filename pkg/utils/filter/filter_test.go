package filter

import (
	"encoding/json"
	"testing"
)

func TestFilter(t *testing.T) {
	filter, err := NewFilterSkip(
		[]string{`out of mem`, `out of mem`, `out of mem`},
		[]string{`Out of memory:`, `(?i)\b(invoked|triggered) oom-killer\b`, `oom-kill:constraint=`},
		[]string{`/var/log/dmesg`},
		[][]string{{"dmesg"}},
		5000,
		80,
	)
	if err != nil {
		t.Fatalf("failed to new file filter:%v", err)
	}
	defer filter.Close()
	result := filter.Check()
	result = append(result, filter.Check()...)

	for i := 0; i < len(result); i++ {
		js, err := json.Marshal(result[i])
		if err != nil {
			t.Fatalf("failed to convert result of FileFilter.Check() to json:%v", err)
		}
		print(string(js) + "\n")
	}
}
