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
package collector

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// execBounded runs name+args capturing combined output, returning after at most
// `timeout` even if the process wedges — e.g. nvidia-smi blocked in a D-state driver
// ioctl on a hung GPU. On timeout it kills the process group and abandons the
// (unkillable) child, returning an error so the caller never blocks forever. This is
// the same non-blocking discipline runProbe applies to the probe binary, applied to
// the gating queries: a wedged GPU is the exact condition gpuprobe must survive, and
// it is precisely what can make nvidia-smi itself hang.
func execBounded(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan error, 1) // buffered: abandoned Wait goroutine can still send and exit
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return buf.String(), err // Wait returned → buf has no concurrent writer
	case <-timer.C:
		killProcessGroup(cmd)
		return "", fmt.Errorf("%s timed out after %s (possible hung GPU)", name, timeout)
	case <-ctx.Done():
		killProcessGroup(cmd)
		return "", ctx.Err()
	}
}

// runProbe execs the probe on one GPU, classifies the exit code, and reclaims the
// child rigorously. On timeout it kills the whole process group and waits at most
// killGraceSec for reaping; a D-state hang that can't be reaped is abandoned (the
// kernel reaps it when the stuck kernel returns) so the daemon is never wedged.
func runProbe(ctx context.Context, binPath string, idx, minFreePct, timeoutSec, killGraceSec int) (outcome string, exitCode int, detail string, durMs int64) {
	start := time.Now()

	cmd := exec.Command(binPath, "-d", strconv.Itoa(idx), "--min-free-pct", strconv.Itoa(minFreePct))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own process group → kill children too
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return OutcomeExecErr, -1, "failed to exec probe " + binPath + ": " + err.Error(), time.Since(start).Milliseconds()
	}

	done := make(chan error, 1) // buffered: abandoned Wait goroutine can still send and exit
	go func() { done <- cmd.Wait() }()

	var timeoutCh <-chan time.Time
	if timeoutSec > 0 {
		t := time.NewTimer(time.Duration(timeoutSec) * time.Second)
		defer t.Stop()
		timeoutCh = t.C
	}

	select {
	case err := <-done:
		durMs = time.Since(start).Milliseconds()
		detail = strings.TrimSpace(buf.String())
		return classifyExit(err, detail, durMs)
	case <-timeoutCh:
		killProcessGroup(cmd)
		reaped := reapWithGrace(done, killGraceSec, idx)
		durMs = time.Since(start).Milliseconds()
		detail = "probe timed out after " + strconv.Itoa(timeoutSec) + "s"
		if reaped { // only safe to read buf once Wait() returned (no concurrent writer)
			detail += ": " + strings.TrimSpace(buf.String())
		}
		return OutcomeTimeout, -1, detail, durMs
	case <-ctx.Done():
		killProcessGroup(cmd)
		reapWithGrace(done, killGraceSec, idx)
		return OutcomeExecErr, -1, "probe canceled: " + ctx.Err().Error(), time.Since(start).Milliseconds()
	}
}

func classifyExit(err error, detail string, durMs int64) (string, int, string, int64) {
	if err == nil {
		return OutcomePass, 0, detail, durMs
	}
	if ee, ok := err.(*exec.ExitError); ok {
		switch ee.ExitCode() {
		case 1:
			return OutcomeFail, 1, detail, durMs
		case 2:
			return OutcomeSkip, 2, detail, durMs
		case 3:
			return OutcomeEnvErr, 3, detail, durMs
		default:
			return OutcomeEnvErr, ee.ExitCode(), "unexpected exit code: " + detail, durMs
		}
	}
	return OutcomeExecErr, -1, "probe error: " + err.Error() + " " + detail, durMs
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // negative pid = whole group
		_ = cmd.Process.Kill()
	}
}

// reapWithGrace waits up to graceSec for the (killed) child to be reaped. Returns
// true if reaped. On false (D-state hang) it abandons the wait; the Wait goroutine
// stays parked on the buffered `done` channel and exits whenever the kernel finally
// releases the process — never blocking the caller.
func reapWithGrace(done chan error, graceSec, idx int) bool {
	timer := time.NewTimer(time.Duration(graceSec) * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		logrus.WithField("component", "gpuprobe").Warnf(
			"probe on GPU %d unkillable within %ds grace (likely D-state hang); abandoning wait", idx, graceSec)
		return false
	}
}
