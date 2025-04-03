package k8s

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1alpha1"
)

type PodResourceMapper struct {
	Name                          string
	PodResourcesKubeletSocketPath string
	ConnectionTimeout             time.Duration
	callMtx                       sync.RWMutex
}

var (
	podResourceMapper *PodResourceMapper
	syncOnce          sync.Once
)

func NewPodResourceMapper() *PodResourceMapper {
	syncOnce.Do(func() {
		var err error
		podResourceMapper, err = newPodResourceMapper()
		if err != nil {
			panic(err)
		}
	})
	return podResourceMapper
}

func newPodResourceMapper() (*PodResourceMapper, error) {
	socketPath := "/var/lib/kubelet/pod-resources/kubelet.sock"
	_, err := os.Stat(socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no Kubelet socket")
		}
		return nil, fmt.Errorf("error checking Kubelet socket: %v", err)
	}

	return &PodResourceMapper{
		Name:                          "PodResourceMapper",
		ConnectionTimeout:             10 * time.Second,
		PodResourcesKubeletSocketPath: socketPath,
		callMtx:                       sync.RWMutex{},
	}, nil
}

func (p *PodResourceMapper) GetDeviceToPodMap() (map[string]string, error) {
	p.callMtx.Lock()
	defer p.callMtx.Unlock()
	// Context with timeout for the request
	ctx, cancel := context.WithTimeout(context.Background(), p.ConnectionTimeout)
	defer cancel()

	// Create a new gRPC client
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	client, err := grpc.NewClient("unix://"+p.PodResourcesKubeletSocketPath, opts...)
	if err != nil {
		logrus.Errorf("Failed to create gRPC client: %v", err)
		return nil, err
	}
	defer func(client *grpc.ClientConn) {
		err := client.Close()
		if err != nil {
			logrus.Errorf("Failed to close gRPC client: %v", err)
		}
	}(client)

	// Create the PodResources client
	prClient := podresourcesapi.NewPodResourcesListerClient(client)

	// List Pod Resources
	resp, err := prClient.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		logrus.Errorf("Failed to getting pod resources: %v", err)
		return nil, err
	}

	// Print pod resources
	deviceToPodMap := make(map[string]string)
	for _, pod := range resp.PodResources {
		// logrus.Infof("Pod: %s/%s\n", pod.Namespace, pod.Name)
		for _, container := range pod.Containers {
			// logrus.Infof("  Container: %s\n", container.Name)
			for _, device := range container.Devices {
				// logrus.Infof("    Resource Name: %s, Devices: %v\n", device.ResourceName, device.DeviceIds)
				if device.ResourceName == "nvidia.com/gpu" {
					for _, deviceID := range device.DeviceIds {
						deviceToPodMap[deviceID] = pod.Name
					}
				}
			}
		}
	}
	if len(deviceToPodMap) == 0 {
		return nil, nil
	}
	return deviceToPodMap, nil
}
