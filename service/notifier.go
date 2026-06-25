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
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/scitix/sichek/pkg/k8s"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

type Notifier interface {
	SendAlert(ctx context.Context, data interface{}) (*http.Response, error)
	// SetNodeAnnotation replaces the item's issues in the node annotation and
	// returns the resulting annotation so callers (e.g. the snapshot) can embed
	// the exact same object that was written to K8s.
	SetNodeAnnotation(ctx context.Context, data *common.Result) (*nodeAnnotation, error)
	// AppendNodeAnnotation accumulates the item's issues (used for
	// HealthCheckTimeout) and returns the resulting annotation.
	AppendNodeAnnotation(ctx context.Context, data *common.Result) (*nodeAnnotation, error)
	// ResetNodeAnnotation clears the entire node annotation and returns the
	// resulting (empty) annotation. Used once at daemon startup so issues
	// written before a restart/reboot do not persist: the annotation lives on
	// the K8s node object across restarts while in-memory state does not, and
	// each component only ever rewrites its own key when it next ticks — so a
	// component that no longer runs would otherwise leave its pre-restart alert
	// forever. Live checks repopulate within one query interval.
	ResetNodeAnnotation(ctx context.Context) (*nodeAnnotation, error)
}

// k8sAnnotationClient is the subset of the K8s client the notifier needs. It is
// an interface so tests can inject an in-memory fake. *k8s.K8sClient satisfies it.
type k8sAnnotationClient interface {
	GetCurrNode(ctx context.Context) (*v1.Node, error)
	UpdateNodeAnnotation(ctx context.Context, anno map[string]string) error
}

type notifier struct {
	client    *http.Client
	k8sClient k8sAnnotationClient

	annoKey         string
	AnnotationMutex sync.Mutex
}

func NewNotifier(annoKey string) (Notifier, error) {
	k8sClient, err := k8s.NewClient()
	if err != nil {
		logrus.Warnf("failed to create K8s client (non-K8s environment?): %v, continuing without K8s annotation support", err)
		k8sClient = nil
	}
	if len(annoKey) == 0 {
		annoKey = consts.DefaultAnnoKey
	}

	n := &notifier{
		client:  &http.Client{},
		annoKey: annoKey,
	}
	// Only assign when non-nil: storing a typed nil (*k8s.K8sClient)(nil) into
	// the interface field would make `n.k8sClient == nil` false and panic later.
	if k8sClient != nil {
		n.k8sClient = k8sClient
	}
	return n, nil
}

func (n *notifier) SendAlert(ctx context.Context, data interface{}) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		logrus.Printf("Error marshaling JSON: %v", err)
		return nil, err
	}
	fmt.Println(string(jsonData))
	return nil, nil
}

func (n *notifier) SetNodeAnnotation(ctx context.Context, data *common.Result) (*nodeAnnotation, error) {
	if n.k8sClient == nil {
		logrus.Debug("K8s client not available, skipping node annotation update")
		return nil, nil
	}
	n.AnnotationMutex.Lock()
	defer n.AnnotationMutex.Unlock()
	node, err := n.k8sClient.GetCurrNode(ctx)
	if err != nil {
		logrus.Errorf("get current node failed: %v", err)
		return nil, err
	}
	anno := parseAnnotationOrEmpty(node.Annotations[n.annoKey])
	err = anno.ParseFromResult(data)
	if err != nil {
		logrus.Errorf("parse annotation from %s result failed: %v", data.Item, err)
		return anno, err
	}
	annoStr, err := anno.JSON()
	if err != nil {
		logrus.Errorf("marshal annotation failed: %v", err)
		return anno, err
	}
	err = n.k8sClient.UpdateNodeAnnotation(ctx, map[string]string{n.annoKey: annoStr})
	if err != nil {
		logrus.Errorf("update node annotation to %s failed: %v", annoStr, err)
	}
	return anno, err
}

func (n *notifier) ResetNodeAnnotation(ctx context.Context) (*nodeAnnotation, error) {
	if n.k8sClient == nil {
		logrus.Debug("K8s client not available, skipping node annotation reset")
		return nil, nil
	}
	n.AnnotationMutex.Lock()
	defer n.AnnotationMutex.Unlock()

	empty := &nodeAnnotation{}
	annoStr, err := empty.JSON()
	if err != nil {
		logrus.Errorf("marshal empty annotation failed: %v", err)
		return nil, err
	}
	if err := n.k8sClient.UpdateNodeAnnotation(ctx, map[string]string{n.annoKey: annoStr}); err != nil {
		logrus.Errorf("reset node annotation failed: %v", err)
		return empty, err
	}
	logrus.WithField("daemon", "run").Info("reset node annotation on startup to clear pre-restart issues")
	return empty, nil
}

func (n *notifier) AppendNodeAnnotation(ctx context.Context, data *common.Result) (*nodeAnnotation, error) {
	if n.k8sClient == nil {
		logrus.Debug("K8s client not available, skipping node annotation update")
		return nil, nil
	}
	n.AnnotationMutex.Lock()
	defer n.AnnotationMutex.Unlock()
	node, err := n.k8sClient.GetCurrNode(ctx)
	if err != nil {
		logrus.Errorf("get current node failed: %v", err)
		return nil, err
	}
	anno := parseAnnotationOrEmpty(node.Annotations[n.annoKey])
	err = anno.AppendFromResult(data)
	if err != nil {
		logrus.Errorf("parse annotation from %s result failed: %v", data.Item, err)
		return anno, err
	}
	annoStr, err := anno.JSON()
	if err != nil {
		logrus.Errorf("marshal annotation failed: %v", err)
		return anno, err
	}
	err = n.k8sClient.UpdateNodeAnnotation(ctx, map[string]string{n.annoKey: annoStr})
	if err != nil {
		logrus.Errorf("update node annotation to %s failed: %v", annoStr, err)
	}
	return anno, err
}
