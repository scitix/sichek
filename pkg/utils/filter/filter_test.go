/*
Copyright 2024 The Scitix Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
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
