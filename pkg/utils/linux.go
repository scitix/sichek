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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
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
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, nil, fmt.Errorf("command `%s %v` on host timed out", command, args)
		}
		return out.Bytes(), stderr.Bytes(), fmt.Errorf("failed to execute command `%s %v` on host: err=%s", command, args, err.Error())
	}
	return out.Bytes(), stderr.Bytes(), nil
}

// TimeTrack Measure execution time of a function
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	fmt.Printf("%s took %d ns\n", name, elapsed.Nanoseconds())
}

func IsNvidiaGPUExist() bool {
	// Check if at least one actual GPU device exists
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		return true
	}
	return false
}

func IsInfinibandExist() bool {
	const dir = "/sys/class/infiniband"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// IsLowSpeedIBBond returns true if the given IB device name looks like a bond
// (contains "bond") and its IB port rate is <= 100 Gb/sec, which indicates a
// non-HCA management bond (e.g. two 25G NIC slaves aggregated) rather than a
// business RoCE LAG / IB bond over high-speed HCAs.
//
// Detection uses /sys/class/infiniband/<ibDev>/ports/1/rate — available
// inside containers that expose /sys/class/infiniband — instead of the
// ethernet-bond slave speed path which is often not visible there.
//
// When the rate file cannot be read or parsed, returns false so callers
// conservatively keep the device rather than silently dropping a potential
// business port.
func IsLowSpeedIBBond(ibDev string) bool {
	if !strings.Contains(ibDev, "bond") {
		return false
	}
	ratePath := filepath.Join("/sys/class/infiniband", ibDev, "ports/1/rate")
	content, err := os.ReadFile(ratePath)
	if err != nil {
		logrus.WithField("component", "utils").Debugf("unable to read IB rate for %s at %s: %v, not treating as low-speed bond", ibDev, ratePath, err)
		return false
	}
	// rate file format e.g. "200 Gb/sec (2X NDR)"
	fields := strings.Fields(strings.TrimSpace(string(content)))
	if len(fields) == 0 {
		return false
	}
	gbps, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		logrus.WithField("component", "utils").Debugf("failed to parse IB rate %q for %s: %v", string(content), ibDev, err)
		return false
	}
	if gbps <= 100 {
		logrus.WithField("component", "utils").Infof("IB bond %s port rate %.0f Gb/sec <= 100, treating as non-HCA management bond", ibDev, gbps)
		return true
	}
	return false
}

