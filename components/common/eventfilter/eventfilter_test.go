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
package eventfilter

import (
	"fmt"
	"os"
	"testing"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventFilter(t *testing.T) {
	type testCase struct {
		name         string
		skipPercent  int64
		rules        map[string]*common.EventRuleConfig
		expectedHits []string // expected checker names
	}

	// 准备测试日志内容
	tempFile, err := os.CreateTemp("", "test.log")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	for i := 0; i < 13; i++ {
		_, err := tempFile.WriteString(fmt.Sprintf("test%d log data\n", i%10))
		require.NoError(t, err)
	}
	require.NoError(t, tempFile.Close())

	tests := []testCase{
		{
			name:        "match test2 at 10% offset",
			skipPercent: 10,
			rules: map[string]*common.EventRuleConfig{
				"test2": {
					Name:        "test2",
					LogFile:     tempFile.Name(),
					Regexp:      "test2",
					Description: "desc2", Level: consts.LevelInfo, Suggestion: "sug2",
				},
				"test10": { // 不匹配
					Name:        "test10",
					LogFile:     tempFile.Name(),
					Regexp:      "test10",
					Description: "desc10", Level: consts.LevelWarning, Suggestion: "sug10",
				},
			},
			expectedHits: []string{"test2"},
		},
		{
			name:        "match test9 at 90% offset",
			skipPercent: 60,
			rules: map[string]*common.EventRuleConfig{
				"test4": {
					Name:        "test4",
					LogFile:     tempFile.Name(),
					Regexp:      "test4",
					Description: "desc4", Level: consts.LevelInfo, Suggestion: "sug4",
				},
				"test9": {
					Name:        "test9",
					LogFile:     tempFile.Name(),
					Regexp:      "test9",
					Description: "desc9", Level: consts.LevelWarning, Suggestion: "sug9",
				},
			},
			expectedHits: []string{"test9"},
		},
		{
			name:        "match both test2 and test9",
			skipPercent: 10,
			rules: map[string]*common.EventRuleConfig{
				"test2": {
					Name:        "test2",
					LogFile:     tempFile.Name(),
					Regexp:      "test2",
					Description: "desc2", Level: consts.LevelInfo, Suggestion: "sug2",
				},
				"test9": {
					Name:        "test9",
					LogFile:     tempFile.Name(),
					Regexp:      "test9",
					Description: "desc9", Level: consts.LevelWarning, Suggestion: "sug9",
				},
			},
			expectedHits: []string{"test2", "test9"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter, err := NewEventFilter("testFilter", tc.rules, tc.skipPercent)
			require.NoError(t, err)
			require.NotNil(t, filter)

			assert.Len(t, filter.RegexEntries, len(tc.rules))
			assert.Len(t, filter.FileEntryMap, 1) // all rules use same file

			result := filter.Check()
			var checkerNames []string
			for _, checker := range result.Checkers {
				checkerNames = append(checkerNames, checker.Name)
			}
			assert.ElementsMatch(t, tc.expectedHits, checkerNames)

			jsonData, err := result.JSON()
			assert.NoError(t, err)
			t.Logf("Result JSON: %s", jsonData)
			filter.Close()
		})
	}
}
