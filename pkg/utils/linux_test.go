package utils

import (
	"context"
	"errors"
	"strings"
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

func TestExecCommandWithContext_fm(t *testing.T) {
	ctx := context.Background()

	// disable perfomance mode for testing
	t.Logf("======test: `systemctl stop nvidia-fabricmanager`=====")
	output, err := ExecCommand(ctx, "systemctl", "stop", "nvidia-fabricmanager")
	if err != nil {
		if strings.Contains(string(output), "nvidia-fabricmanager.service not loaded") ||
			strings.Contains(string(output), "Failed to connect to bus") { // skip for gitlab-ci
			t.Skipf("command `systemctl stop nvidia-fabricmanager`: output= %v, err=%s", string(output), err.Error())
			return
		} else {
			t.Fatalf("failed to stop nvidia-fabricmanager: %v, output: %v", err, string(output))
		}
	}
	t.Logf("======test: `systemctl status nvidia-fabricmanager`=====")
	output, _ = ExecCommand(ctx, "systemctl", "status", "nvidia-fabricmanager")
	t.Logf("nvidia-fabricmanager status: %s", string(output))

	t.Logf("======test: `systemctl is-active nvidia-fabricmanager`=====")
	output, err = ExecCommand(ctx, "systemctl", "is-active", "nvidia-fabricmanager")
	if err != nil {
		if strings.Contains(string(output), "inactive") {
			t.Logf("command `systemctl is-active nvidia-fabricmanager`: output= %v, err=%s", string(output), err.Error())
		} else {
			t.Fatalf("expected an error, got output=%v: %v", string(output), err)
		}
	}
}
