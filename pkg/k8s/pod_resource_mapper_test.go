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
	for deviceID, podName := range deviceToPodMap {
		logrus.Infof("Device: %s, Pod: %s\n", deviceID, podName)
	}
}
