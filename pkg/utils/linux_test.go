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
package utils

import (
	"context"
	"errors"
	// "strings"
	"testing"
	"time"
)

func TestExecCommandWithContext_Success(t *testing.T) {
	ctx := context.Background()
	command := "echo"
	args := []string{"hello"}

	output, err := ExecCommand(ctx, command, args...)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedOutput := "hello\n"
	if string(output) != expectedOutput {
		t.Fatalf("expected %q, got %q", expectedOutput, output)
	}
}

func TestExecCommandWithContext_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	command := "sleep"
	args := []string{"1"}

	_, err := ExecCommand(ctx, command, args...)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", ctx.Err())
	}
}

// TestExecCommandWithContext_TimeoutKillsRunningProcess verifies that a
// genuinely long-running command is killed when the context deadline fires and
// that ExecCommand returns close to the deadline rather than waiting for the
// command to finish on its own. This is the load-bearing property behind the
// bounded nvidia-smi startup probe: a hung `nvidia-smi -q` must be terminated
// at the probe timeout instead of blocking daemon startup indefinitely.
func TestExecCommandWithContext_TimeoutKillsRunningProcess(t *testing.T) {
	if IsRunningInKubernetes() {
		t.Skip("skipping: ExecCommand routes through nsenter under Kubernetes")
	}
	const deadline = 300 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	start := time.Now()
	// `sleep 10` would run far longer than the deadline if not killed.
	_, err := ExecCommand(ctx, "sleep", "10")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	// Must return shortly after the deadline, not after the full sleep.
	if elapsed > 3*time.Second {
		t.Fatalf("ExecCommand did not return promptly after deadline: elapsed=%v (want <3s)", elapsed)
	}
}

func TestExecCommandWithContext_CommandError(t *testing.T) {
	ctx := context.Background()
	command := "false" // `false` command always returns a non-zero exit status

	_, err := ExecCommand(ctx, command)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestExecCommandWithContext_OfedInfo(t *testing.T) {
	ctx := context.Background()

	output, err := ExecCommand(ctx, "ofed_info", "-s")
	if err != nil {
		t.Fatalf("expected an error, got output=%v: %v", output, err)
	}
}

// func TestExecCommandWithContext_fm(t *testing.T) {
// 	ctx := context.Background()

// 	// disable perfomance mode for testing
// 	t.Logf("======test: `systemctl stop nvidia-fabricmanager`=====")
// 	output, err := ExecCommand(ctx, "systemctl", "stop", "nvidia-fabricmanager")
// 	if err != nil {
// 		if strings.Contains(string(output), "nvidia-fabricmanager.service not loaded") ||
// 			strings.Contains(string(output), "Failed to connect to bus") { // skip for gitlab-ci
// 			t.Skipf("command `systemctl stop nvidia-fabricmanager`: output= %v, err=%s", string(output), err.Error())
// 			return
// 		} else {
// 			t.Fatalf("failed to stop nvidia-fabricmanager: %v, output: %v", err, string(output))
// 		}
// 	}
// 	t.Logf("======test: `systemctl status nvidia-fabricmanager`=====")
// 	output, _ = ExecCommand(ctx, "systemctl", "status", "nvidia-fabricmanager")
// 	t.Logf("nvidia-fabricmanager status: %s", string(output))

// 	t.Logf("======test: `systemctl is-active nvidia-fabricmanager`=====")
// 	output, err = ExecCommand(ctx, "systemctl", "is-active", "nvidia-fabricmanager")
// 	if err != nil {
// 		if strings.Contains(string(output), "inactive") {
// 			t.Logf("command `systemctl is-active nvidia-fabricmanager`: output= %v, err=%s", string(output), err.Error())
// 		} else {
// 			t.Fatalf("expected an error, got output=%v: %v", string(output), err)
// 		}
// 	}
// }

func TestParseUptime(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    float64
		wantErr bool
	}{
		{name: "normal", content: "12345.67 98765.43\n", want: 12345.67},
		{name: "single field", content: "42.0", want: 42.0},
		{name: "leading spaces", content: "   7.5   1.2\n", want: 7.5},
		{name: "empty", content: "", wantErr: true},
		{name: "whitespace only", content: "   \n", wantErr: true},
		{name: "non-numeric", content: "abc def", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUptime(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value %v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseUptime(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

func TestGetUptime(t *testing.T) {
	uptime, err := GetUptime()
	if err != nil {
		t.Fatalf("GetUptime() error: %v", err)
	}
	if uptime <= 0 {
		t.Fatalf("GetUptime() = %v, want > 0", uptime)
	}
}

func TestGetBootTime(t *testing.T) {
	before := time.Now()
	boot, err := GetBootTime()
	if err != nil {
		t.Fatalf("GetBootTime() error: %v", err)
	}
	if !boot.Before(before) {
		t.Fatalf("GetBootTime() = %v, want a time in the past (before %v)", boot, before)
	}
	// boot ≈ now - uptime; cross-check against a fresh uptime read within a few seconds.
	uptime, err := GetUptime()
	if err != nil {
		t.Fatalf("GetUptime() error: %v", err)
	}
	expected := time.Now().Add(-time.Duration(uptime * float64(time.Second)))
	if diff := boot.Sub(expected); diff > 5*time.Second || diff < -5*time.Second {
		t.Fatalf("GetBootTime() = %v, want ≈ %v (diff %v)", boot, expected, diff)
	}
}
