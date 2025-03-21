package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

func IsRoot() bool {
	return os.Geteuid() == 0
}

func IsRunningInKubernetes() bool {
	_, hasServiceHost := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	_, hasPort := os.LookupEnv("KUBERNETES_PORT")
	return hasServiceHost && hasPort
}

func ExecCommand(ctx context.Context, command string, args ...string) ([]byte, error) {
	if IsRunningInKubernetes() {
		output, stderr, err := execOnHost(ctx, command, args...)
		if err != nil {
			// `which` returns a non-zero exit code if the command is not found
			if len(stderr) > 0 {
				return stderr, err
			}
			return output, err
		}
		return output, err
	} else {
		return execCommandWithContext(ctx, command, args...)
	}
}

func execCommandWithContext(ctx context.Context, command string, args ...string) ([]byte, error) {
	// Use exec.CommandContext to create a command with the context, Run the command and capture the output
	output, err := exec.CommandContext(ctx, command, args...).CombinedOutput()
	if err != nil {
		// Check if the context was canceled or timed out
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command `%s %v` timed out", command, args)
		}
		return output, fmt.Errorf("failed to execute command `%s %v`: err=%s", command, args, err.Error())
	}
	return output, nil
}

func execOnHost(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	if command == "" {
		return nil, nil, fmt.Errorf("command cannot be empty")
	}

	if _, err := exec.LookPath("nsenter"); err != nil {
		return nil, nil, fmt.Errorf("nsenter not found: %v", err)
	}

	// Prepare the nsenter command
	nsenterArgs := []string{
		"--mount=" + "/proc/1/ns/mnt",
		"--",
		command,
}

	nsenterArgs = append(nsenterArgs, args...)

	// Create the command
	cmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)

	// Capture the output
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	// Run the command
	// Note: systemctl status inactive nvidia-fabricmanager returns exit status 3
	err := cmd.Run()
	if err != nil {
		// Check if the context was canceled or timed out
		if ctx.Err() == context.DeadlineExceeded {
			return nil, nil, fmt.Errorf("command `%s %v` on host timed out", command, args)
		}
		return out.Bytes(), stderr.Bytes(), fmt.Errorf("failed to execute command `%s %v` on host: err=%s", command, args, err.Error())
	}
	return out.Bytes(), stderr.Bytes(), nil
}

// Measure execution time of a function
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %d ns\n", name, elapsed.Nanoseconds())
}

func IsNvidiaGPUExist() bool {
  // Check if the server is GPU server
	matches, err := filepath.Glob("/dev/nvidia*")
	if err != nil {
		logrus.WithField("component", "utils").Infof("Fail to run the cmd: filepath.Glob, err = %v; treat as not GPU server", err)
		return false
	}
	return len(matches) > 0
}
