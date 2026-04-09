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

// IsManagementBond checks if the given IB device name corresponds to a management network bond.
// It is considered a management bond if its network member interface(s) have a speed <= 100G (100000 Mbps).
func IsManagementBond(devName string) bool {
	bondName := devName
	// e.g. "mlx5_bond_0" -> "bond0"
	if strings.HasPrefix(devName, "mlx5_bond_") {
		bondName = "bond" + strings.TrimPrefix(devName, "mlx5_bond_")
	} else if strings.HasPrefix(devName, "roce_bond") {
		bondName = "bond" + strings.TrimPrefix(devName, "roce_bond")
	} else if !strings.HasPrefix(devName, "bond") {
		idx := strings.Index(devName, "bond")
		if idx != -1 {
			bondName = devName[idx:]
		}
	}

	logrus.WithFields(logrus.Fields{
		"component": "utils",
		"devName":   devName,
		"bondName":  bondName,
	}).Debugf("checking if device is a management bond")

	slavesPath := filepath.Join("/sys/class/net", bondName, "bonding", "slaves")
	content, err := os.ReadFile(slavesPath)
	if err != nil {
		logrus.WithField("component", "utils").Debugf("unable to read slaves for bond %s at %s: %v, assuming not a management bond", bondName, slavesPath, err)
		return false // unable to read slaves, assume it's not a management bond
	}

	slaves := strings.Fields(strings.TrimSpace(string(content)))
	if len(slaves) == 0 {
		logrus.WithField("component", "utils").Debugf("bond %s has no slaves", bondName)
		return false
	}

	logrus.WithField("component", "utils").Debugf("bond %s has slaves: %v", bondName, slaves)

	for _, slave := range slaves {
		speedPath := filepath.Join("/sys/class/net", slave, "speed")
		speedStr, err := os.ReadFile(speedPath)
		if err != nil {
			logrus.WithField("component", "utils").Debugf("unable to read speed for slave %s at %s: %v", slave, speedPath, err)
			continue
		}

		speed, err := strconv.Atoi(strings.TrimSpace(string(speedStr)))
		if err != nil {
			logrus.WithField("component", "utils").Debugf("failed to parse speed for slave %s: %v", slave, err)
			continue
		}

		logrus.WithField("component", "utils").Debugf("slave %s speed is %d Mbps", slave, speed)

		// 100G is 100000 Mbps
		if speed <= 100000 {
			logrus.WithField("component", "utils").Infof("device %s (bond %s, slave %s) speed is <= 100000 Mbps, identified as management bond", devName, bondName, slave)
			return true
		}
	}

	return false
}
