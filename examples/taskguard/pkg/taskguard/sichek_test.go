// sichek_test.go
package taskguard

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scitix/taskguard/pkg/cfg"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

// fakeNodeInformer is a mock implementation of a node informer
type fakeNodeInformer struct {
	store cache.Store
}

// Add required SharedInformer interface methods
func (f *fakeNodeInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}
func (f *fakeNodeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}
func (f *fakeNodeInformer) GetController() cache.Controller                            { return nil }
func (f *fakeNodeInformer) Run(stopCh <-chan struct{})                                 {}
func (f *fakeNodeInformer) HasSynced() bool                                            { return true }
func (f *fakeNodeInformer) LastSyncResourceVersion() string                            { return "" }
func (f *fakeNodeInformer) SetWatchErrorHandler(handler cache.WatchErrorHandler) error { return nil }
func (f *fakeNodeInformer) SetTransform(handler cache.TransformFunc) error             { return nil }
func (f *fakeNodeInformer) IsStopped() bool                                            { return false }
func (f *fakeNodeInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return nil
}

func (f *fakeNodeInformer) GetStore() cache.Store {
	return f.store
}

func TestIsTaskPodHealthy(t *testing.T) {
	controller := &Controller{
		nodeInformer: &fakeNodeInformer{
			store: cache.NewStore(cache.MetaNamespaceKeyFunc),
		},
		config: cfg.FaultToleranceConfig{
			SiChekNodeAnnotationKey: "scitix.ai/sicheck",
		},
	}

	// Create a fake node with annotations
	node := &corev1.Node{}
	sicheckTest := SiChekResult{
		NCCL: map[string][]*annotation{
			"fatal": []*annotation{
				&annotation{
					ErrorName: "test_err",
				},
			},
		},
	}
	jsonString, _ := json.Marshal(sicheckTest)
	node.Annotations = map[string]string{
		"scitix.ai/sicheck": string(jsonString),
	}
	node.Name = "test-node"
	controller.nodeInformer.GetStore().Add(node)

	isHealthy := controller.isTaskPodHealthy("test-node", "test-pod")
	assert.False(t, isHealthy)
}

func TestIsTaskPodHangFromSiChek(t *testing.T) {
	controller := &Controller{
		nodeInformer: &fakeNodeInformer{
			store: cache.NewStore(cache.MetaNamespaceKeyFunc),
		},
		config: cfg.FaultToleranceConfig{},
	}

	// Create a fake node with annotations
	node := &corev1.Node{}
	sicheckTest := SiChekResult{
		Hang: map[string][]*annotation{
			"fatal": []*annotation{
				&annotation{
					ErrorName: "test_hang",
					Devcie:    "gpu1:test-pod",
				},
			},
		},
	}
	jsonString, _ := json.Marshal(sicheckTest)
	node.Annotations = map[string]string{
		"scitix.ai/sicheck": string(jsonString),
	}
	node.Name = "test-node"
	controller.nodeInformer.GetStore().Add(node)

	isHang := controller.isTaskPodHangFromSiChek("test-node", "test-pod")
	assert.False(t, isHang)
}
