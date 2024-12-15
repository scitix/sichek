package utils

import (
	"context"
	"testing"
	"time"
)

func TestExecCommandWithContext_Success(t *testing.T) {
	ctx := context.Background()
	command := "echo"
	args := []string{"hello"}

	output, err := execCommandWithContext(ctx, command, args...)
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

	_, err := execCommandWithContext(ctx, command, args...)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded, got %v", ctx.Err())
	}
}

func TestExecCommandWithContext_CommandError(t *testing.T) {
	ctx := context.Background()
	command := "false" // `false` command always returns a non-zero exit status

	_, err := execCommandWithContext(ctx, command)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
