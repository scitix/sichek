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
)

type Notifier interface {
	SendAlert(ctx context.Context, data interface{}) (*http.Response, error)
	SetNodeAnnotation(ctx context.Context, data *common.Result) error
	AppendNodeAnnotation(ctx context.Context, data *common.Result) error
}

type notifier struct {
	client    *http.Client
	k8sClient *k8s.K8sClient

	endpoint        string
	port            int
	annoKey         string
	AnnotationMutex sync.Mutex
}

func NewNotifier(annoKey string) (Notifier, error) {
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return nil, err
	}
	if len(annoKey) == 0 {
		annoKey = consts.DefaultAnnoKey
	}

	return &notifier{
		client:    &http.Client{},
		k8sClient: k8sClient,
		endpoint:  consts.TaskGuardEndpoint,
		port:      consts.TaskGuardPort,
		annoKey:   annoKey,
	}, nil
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

func (n *notifier) SetNodeAnnotation(ctx context.Context, data *common.Result) error {
	n.AnnotationMutex.Lock()
	defer n.AnnotationMutex.Unlock()
	node, err := n.k8sClient.GetCurrNode(ctx)
	if err != nil {
		logrus.Errorf("get current node failed: %v", err)
		return err
	}
	anno, err := GetAnnotationFromJson(node.Annotations[n.annoKey])
	if err != nil {
		logrus.Errorf("parse annotation %s failed: %v", node.Annotations[n.annoKey], err)
		return err
	}
	err = anno.ParseFromResult(data)
	if err != nil {
		logrus.Errorf("parse annotation from %s result failed: %v", data.Item, err)
		return err
	}
	annoStr, err := anno.JSON()
	if err != nil {
		logrus.Errorf("marshal annotation failed: %v", err)
		return err
	}
	err = n.k8sClient.UpdateNodeAnnotation(ctx, map[string]string{n.annoKey: annoStr})
	if err != nil {
		logrus.Errorf("update node annotation to %s failed: %v", annoStr, err)
	}
	return err
}

func (n *notifier) AppendNodeAnnotation(ctx context.Context, data *common.Result) error {
	n.AnnotationMutex.Lock()
	defer n.AnnotationMutex.Unlock()
	node, err := n.k8sClient.GetCurrNode(ctx)
	if err != nil {
		logrus.Errorf("get current node failed: %v", err)
		return err
	}
	anno, err := GetAnnotationFromJson(node.Annotations[n.annoKey])
	if err != nil {
		logrus.Errorf("parse annotation %s failed: %v", node.Annotations[n.annoKey], err)
		return err
	}
	err = anno.AppendFromResult(data)
	if err != nil {
		logrus.Errorf("parse annotation from %s result failed: %v", data.Item, err)
		return err
	}
	annoStr, err := anno.JSON()
	if err != nil {
		logrus.Errorf("marshal annotation failed: %v", err)
		return err
	}
	err = n.k8sClient.UpdateNodeAnnotation(ctx, map[string]string{n.annoKey: annoStr})
	if err != nil {
		logrus.Errorf("update node annotation to %s failed: %v", annoStr, err)
	}
	return err
}
