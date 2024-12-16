package systemd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/scitix/sichek/pkg/utils"
)

func SystemdExists() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if utils.IsRunningInKubernetes() {
		output, err := utils.ExecCommand(ctx, "which", "systemd")
		if err != nil {
			return false, err
		}
		return len(output) > 0, nil
	} else {
		p, err := exec.LookPath("systemd")
		if err != nil {
			return false, err
		}
		return p != "", nil
	}
}

func SystemctlExists() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if utils.IsRunningInKubernetes() {
		output, err := utils.ExecCommand(ctx, "which", "systemctl")
		if err != nil {
			return false, err
		}
		return len(output) > 0, nil
	} else {
		p, err := exec.LookPath("systemctl")
		if err != nil {
			return false, err
		}
		return p != "", nil
	}
}

func DaemonReload(ctx context.Context) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exist, err := SystemctlExists()
	if !exist {
		return nil, fmt.Errorf("systemctl not exist: %v", err)
	}
	output, err := utils.ExecCommand(cctx, "systemctl", "daemon-reload")
	if err != nil {
		return nil, err
	}
	return output, nil
}

// IsActive returns true if the systemd service is active.
func IsActive(service string) (bool, error) {
	exist, err := SystemctlExists()
	if !exist {
		return false, fmt.Errorf("systemd active check requires systemctl (%w)", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := utils.ExecCommand(ctx, "systemctl", "is-active", service)
	if err != nil {
		// e.g., "inactive" with exit status 3
		if strings.Contains(string(output), "inactive") {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(output)) == "active", nil
}

func EnableSystemdService(service string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exist, err := SystemctlExists()
	if !exist {
		return fmt.Errorf("systemd enable service requires systemctl (%w)", err)
	}

	if out, err := utils.ExecCommand(ctx, "systemctl", "enable", service); err != nil {
		return fmt.Errorf("systemctl enable failed: %w output: %s", err, out)
	}
	return nil
}

func DisableSystemdService(service string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exist, err := SystemctlExists()
	if !exist {
		return fmt.Errorf("systemd disable service requires systemctl (%w)", err)
	}

	if out, err := utils.ExecCommand(ctx, "systemctl", "disable", service); err != nil {
		return fmt.Errorf("systemctl disable failed: %w output: %s", err, out)
	}
	return nil
}

func RestartSystemdService(service string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exist, err := SystemctlExists()
	if !exist {
		return fmt.Errorf("systemd disable service requires systemctl (%w)", err)
	}

	if out, err := utils.ExecCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w output: %s", err, out)
	}
	if out, err := utils.ExecCommand(ctx, "systemctl", "restart", service); err != nil {
		return fmt.Errorf("systemctl restart failed: %w output: %s", err, out)
	}
	return nil
}

func StopSystemdService(service string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exist, err := SystemctlExists()
	if !exist {
		return fmt.Errorf("systemd disable service requires systemctl (%w)", err)
	}

	if out, err := utils.ExecCommand(ctx, "systemctl", "stop", service); err != nil {
		return fmt.Errorf("systemctl stop failed: %w output: %s", err, out)
	}
	return nil
}
