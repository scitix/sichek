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
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/scitix/sichek/consts"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	k8sClient     *K8sClient
	k8sClientOnce sync.Once
)

type K8sClient struct {
	kubeconfig string

	client *kubernetes.Clientset
}

func NewClient() (*K8sClient, error) {
	var err error
	var cfg *rest.Config
	var kubeconfigPath string
	k8sClientOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic occurred: %v", r)
			}
		}()
		_, hasServiceHost := os.LookupEnv("KUBERNETES_SERVICE_HOST")
		_, hasPort := os.LookupEnv("KUBERNETES_PORT")
		if hasServiceHost && hasPort {
			cfg, err = rest.InClusterConfig()
			if err != nil {
				logrus.Warnf("build in-cluster kubeconfig failed (non-K8s environment?): %v", err)
				return
			}
		} else {
			kubeconfigPath = os.Getenv("KUBECONFIG")
			if kubeconfigPath == "" {
				kubeconfigPath = consts.KubeConfigPath
			}
			cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				logrus.Warnf("get kubeconfig failed (non-K8s environment?): %v", err)
				return
			}
		}

		cli, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			logrus.Warnf("NewForConfig failed (non-K8s environment?): %v", err)
			return
		}
		k8sClient = &K8sClient{
			kubeconfig: kubeconfigPath,
			client:     cli,
		}
	})
	if err != nil {
		return nil, err
	}
	return k8sClient, nil
}

func (kc *K8sClient) KubeConfig() string {
	return kc.kubeconfig
}

func (kc *K8sClient) GetCurrNode(ctx context.Context) (*v1.Node, error) {
	nodeName, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}

	node, err := kc.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get k8s node %s failed: %v", nodeName, err)
	}
	return node, nil
}

func (kc *K8sClient) UpdateNodeAnnotation(ctx context.Context, anno map[string]string) error {
	node, err := kc.GetCurrNode(ctx)
	if err != nil {
		return err
	}

	updated := false
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
		updated = true
	}
	for key, value := range anno {
		origin, exist := node.Annotations[key]
		if exist && origin == value {
			continue
		}
		node.Annotations[key] = value
		updated = true
	}

	if !updated {
		return nil
	}

	_, err = kc.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
