package systemd

import "testing"

func TestIsSystemctlExists(t *testing.T) {
	exist, err := SystemctlExists()
	if err != nil {
		t.Errorf("failed to check systemctl exists: %v", err)
	}
	if !exist {
		t.Errorf("systemctl not exists")
	}
}

func TestIsServiceActive(t *testing.T) {
	active, err := IsActive("nvidia-fabricmanager")
	if err != nil {
		t.Errorf("failed to check nvidia-fabricmanager active: %v", err)
	}
	if !active {
		t.Errorf("service nvidia-fabricmanager not active")
	}
}

func TestStartSystemdService(t *testing.T) {
	err := StopSystemdService("nvidia-fabricmanager")
	if err != nil {
		t.Errorf("failed to stop nvidia-fabricmanager: %v", err)
	}
	err = EnableSystemdService("nvidia-fabricmanager")
	if err != nil {
		t.Errorf("failed to start nvidia-fabricmanager: %v", err)
	}
	active, err := IsActive("nvidia-fabricmanager")
	if err != nil {
		t.Errorf("failed to check nvidia-fabricmanager active: %v", err)
	}
	if !active {
		t.Errorf("service nvidia-fabricmanager not active")
	}
}
