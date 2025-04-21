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
package k8s

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewDevicePodMapper(t *testing.T) {
	mapper := NewPodResourceMapper()
	if mapper == nil {
		t.Fatalf("failed to create DevicePodMapper")
	}
	deviceToPodMap, err := mapper.GetDeviceToPodMap()
	if err != nil {
		t.Fatalf("failed to get device to pod map: %v", err)
	}
	for deviceID, podInfo := range deviceToPodMap {
		logrus.Infof("Device: %s, Pod: %+v\n", deviceID, podInfo)
	}
}
