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
package dmesg

import (
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/dmesg/config"
	"github.com/scitix/sichek/consts"
)

type TestCase struct {
	RuleName      string   // The rule name in default_event_rules.yaml (must match the key in event_checkers)
	MockLogLines  []string // Mock log lines that should match this rule's regexp
	ExpectedCount int      // Expected number of matches
}

// TestEventRulesComprehensive tests all event rules defined in default_event_rules.yaml
//
// This is an extensible test framework that directly tests EventFilter against log file content.
// It does NOT run dmesg HealthCheck, but instead:
//  1. Loads all rules from default_event_rules.yaml
//  2. For each rule, writes mock log content to /tmp/sichek.dmesg.log
//  3. Creates an EventFilter and checks if the rule matches
//
// To add a new rule test:
//  1. Add a new rule to default_event_rules.yaml
//  2. Add a TestCase entry to the testCases map below with:
//     - RuleName: the key from default_event_rules.yaml
//     - MockLogLines: sample log lines that should match the regexp
//     - ExpectedCount: number of lines that should match
//
// Example:
//
//	"NewRule": {
//	    RuleName: "NewRule",
//	    MockLogLines: []string{
//	        "[Mon Jan  1 12:00:00 2024] Some log line matching the regexp",
//	    },
//	    ExpectedCount: 1,
//	},
func TestEventRulesComprehensive(t *testing.T) {
	// Load default event rules
	eventRules, err := config.LoadDefaultEventRules()
	if err != nil {
		t.Fatalf("Failed to load default event rules: %v", err)
	}

	testCases := map[string]TestCase{
		"SysOOM": {
			RuleName: "SysOOM",
			MockLogLines: []string{
				"[Mon Jan  1 12:00:00 2024] Out of memory: Kill process 1234 (test) score 500 or sacrifice child",
				"[Mon Jan  1 12:00:01 2024] Out of memory: Killed process 5678",
			},
			ExpectedCount: 2,
		},
		"CgroupOOM": {
			RuleName: "CgroupOOM",
			MockLogLines: []string{
				"[Mon Jan  1 12:00:00 2024] Memory cgroup out of memory: Killed process 1234",
			},
			ExpectedCount: 1,
		},
		"NVSXID": {
			RuleName: "NVSXID",
			MockLogLines: []string{
				"[Mon Jan  1 12:00:00 2024] NVRM: Xid (PCI:0000:01:00.0): 13, SXid 00000001: 00000013,",
				"[Mon Jan  1 12:00:01 2024] NVRM: Xid (PCI:0000:02:00.0): 31, SXid 00000002: 00000031,",
			},
			ExpectedCount: 2,
		},
		"NCCLSegFault": {
			RuleName: "NCCLSegFault",
			MockLogLines: []string{
				"[Mon Jan  1 12:00:00 2024] test_program[1234]: segfault at 0x12345678 ip 0x87654321 sp 0x7fff12345678 error 4 in libnccl.so.2.19.3",
			},
			ExpectedCount: 1,
		},
		"NvErrResetRequired": {
			RuleName: "NvErrResetRequired",
			MockLogLines: []string{
				"[Mon Jan  1 12:00:00 2024] NVRM: rpcRmApiAlloc_GSP: GspRmAlloc failed: hClient=0xc1d1a616; hParent=0x5c000054; hObject=0x5c00005b; hClass=0x0000c86f; paramsSize=0x00000170; paramsStatus=0x00000062; status=0x00000062",
				"[Mon Jan  1 12:00:01 2024] NVRM: nvAssertOkFailedNoLog: Assertion failed: Reset required [NV_ERR_RESET_REQUIRED] (0x00000062) returned from status @ kernel_channel.c:2874",
				"[Mon Jan  1 12:00:02 2024] NVRM: nvAssertOkFailedNoLog: Assertion failed: Reset required [NV_ERR_RESET_REQUIRED] (0x00000062) returned from _kchannelSendChannelAllocRpc(pKernelChannel, pChannelGpfifoParams, pKernelChannelGroup, bFullSriov) @ kernel_channel.c:936",
			},
			ExpectedCount: 2, // Only the last two lines match the regex
		},
	}

	// Test each rule
	for ruleName, testCase := range testCases {
		t.Run(ruleName, func(t *testing.T) {
			// Find the rule in the loaded rules
			rule, exists := eventRules[ruleName]
			if !exists {
				t.Fatalf("Rule %s not found in default event rules group, please check", ruleName)
			}

			// Pipe: mock streaming kmsg
			pr, pw := io.Pipe()
			defer pw.Close()

			rules := map[string]*common.EventRuleConfig{
				ruleName: rule,
			}
			// KmsgReader（skipPercent=0）
			reader, err := NewKmsgReader(pr, 0)
			if err != nil {
				t.Fatalf("create KmsgReader failed: %v", err)
			}
			eventCache := NewEventCache(rules)

			// Start reader -> EventCache
			reader.Start(func(line string) {
				eventCache.MatchLine(line)
			})
			defer reader.Stop()

			// Write mock log lines (formatted as kmsg format)
			for _, line := range testCase.MockLogLines {
				// Format as kmsg: <pri>,<seq>,<ts>,<flags>;message
				kmsgLine := fmt.Sprintf("6,1,0,-;%s\n", line)
				_, err := pw.Write([]byte(kmsgLine))
				if err != nil {
					t.Fatalf("write pipe failed: %v", err)
				}
			}

			// Small delay to ensure messages are processed
			time.Sleep(300 * time.Millisecond)

			// Close the pipe to signal end of input
			pw.Close()

			// Check the rules
			result := eventCache.Drain()

			// Verify the result
			if result == nil {
				t.Fatal("Drain() returned nil result")
			}

			// Find the checker result for this rule
			// Note: EventCache uses the YAML key (ruleName) as the Name, not rule.Name
			var checkerResult *common.CheckerResult
			for _, checker := range result.Checkers {
				if checker.Name == ruleName {
					checkerResult = checker
					break
				}
			}

			if checkerResult == nil {
				t.Fatalf("Rule %s was not found in check results (expected %d matches).", ruleName, testCase.ExpectedCount)
			}

			// Verify expected count
			actualCount, err := strconv.Atoi(checkerResult.Curr)
			if err != nil {
				t.Fatalf("Failed to parse Curr count: %v", err)
			}
			if actualCount != testCase.ExpectedCount {
				t.Errorf("Rule %s: Expected %d matches, got %d", ruleName, testCase.ExpectedCount, actualCount)
				t.Logf("  Detail: %s", checkerResult.Detail)
			}

			// Verify expected level (read from rule config)
			expectedLevel := rule.Level
			if checkerResult.Level != expectedLevel {
				t.Errorf("Rule %s: Expected level %s (from config), got %s", ruleName, expectedLevel, checkerResult.Level)
			}

			// Verify status (should be abnormal if matches found)
			if testCase.ExpectedCount > 0 {
				if checkerResult.Status != consts.StatusAbnormal {
					t.Errorf("Rule %s: Expected status abnormal (found %d matches), got %s", ruleName, actualCount, checkerResult.Status)
				}
			}

			t.Logf("Rule %s tested successfully: %d matches, level=%s, status=%s", ruleName, actualCount, checkerResult.Level, checkerResult.Status)
		})
	}

	// Test that all rules in default_event_rules.yaml have test cases
	t.Run("AllRulesHaveTestCases", func(t *testing.T) {
		for ruleName := range eventRules {
			if _, exists := testCases[ruleName]; !exists {
				t.Errorf("Error: Rule %s in default_event_rules.yaml does not have a test case. Add it to testCases map.", ruleName)
			}
		}
	})
}
