package utils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
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
				return stderr, fmt.Errorf("execute command `%s %v` on host failed: %s", command, args, err.Error())
			}
			return nil, fmt.Errorf("failed to execute nsenter: %v", err)
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
		return nil, fmt.Errorf("failed to execute command `%s %v`: %s", command, args, err.Error())
	}
	return output, nil
}

func execOnHost(ctx context.Context, command string, args ...string) ([]byte, []byte, error) {
	// Path to the host's /proc/1/ns, which is the namespace of the host's PID 1
	hostProcNamespacePath := "/host/proc/1/ns"

	// Prepare the nsenter command
	nsenterArgs := []string{
		"--target", "1", // Target PID (1 for the host's init process)
		"--mount", "--uts", "--ipc", // Enter mount, UTS, and IPC namespaces
		"--net", "--pid", "--wd=/", // Enter network, PID and root dir namespaces
		"--",    // End of nsenter options
		command, // Command to execute
	}
	nsenterArgs = append(nsenterArgs, args...)

	// Create the command
	cmd := exec.CommandContext(ctx, "nsenter", nsenterArgs...)

	// Set the environment variable to use the host's /proc
	cmd.Env = append(cmd.Env, fmt.Sprintf("PROC_ROOT=%s", hostProcNamespacePath))

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
		return nil, nil, fmt.Errorf("failed to execute command `%s %v`: %s on host", command, args, err.Error())
	}
	return out.Bytes(), stderr.Bytes(), nil
}

// Measure execution time of a function
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %d ns\n", name, elapsed.Nanoseconds())
}
