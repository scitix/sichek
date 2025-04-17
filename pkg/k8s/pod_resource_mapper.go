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

type PodInfo struct {
	Namespace string
	PodName   string
}

// Implement String() method for pretty printing
func (p *PodInfo) String() string {
	return fmt.Sprintf("Namespace: %s, PodName: %s", p.Namespace, p.PodName)
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
			logrus.Errorf("Failed to create PodResourceMapper: %v", err)
		}
	})
	return podResourceMapper
}

func newPodResourceMapper() (*PodResourceMapper, error) {
	socketPath := "/var/lib/kubelet/pod-resources/kubelet.sock"
	var ret error
	ret = nil
	// Check if the socket file exists or is accessible
	_, err := os.Stat(socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			ret = fmt.Errorf("kubelet socket does not exist: %v", err)
		} else {
			ret = fmt.Errorf("kubelet socket is not accessible: %v", err)
		}
		socketPath = ""
	}

	return &PodResourceMapper{
		Name:                          "PodResourceMapper",
		ConnectionTimeout:             10 * time.Second,
		PodResourcesKubeletSocketPath: socketPath,
		callMtx:                       sync.RWMutex{},
	}, ret
}

func (p *PodResourceMapper) GetDeviceToPodMap() (map[string]*PodInfo, error) {
	if p.PodResourcesKubeletSocketPath == "" {
		logrus.Warn("PodResourcesKubeletSocketPath is not set, returning empty map")
		return nil, nil
	}
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
	deviceToPodMap := make(map[string]*PodInfo)
	for _, pod := range resp.PodResources {
		for _, container := range pod.Containers {
			for _, device := range container.Devices {
				if device.ResourceName == "nvidia.com/gpu" {
					for _, deviceID := range device.DeviceIds {
						deviceToPodMap[deviceID] = &PodInfo{
							Namespace: pod.Namespace,
							PodName:   pod.Name,
						}
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
