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
		t.Errorf("failed to enable nvidia-fabricmanager: %v", err)
	}
	err = RestartSystemdService("nvidia-fabricmanager")
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
