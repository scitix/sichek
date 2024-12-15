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
	"log"
	"os"

	"github.com/scitix/taskguard/pkg/cfg"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	CoreClient    *kubernetes.Clientset
	DynamicClient dynamic.Interface
}

func GetKubeConfig(c cfg.KubeConfig) *rest.Config {
	_, hasServiceHost := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	_, hasPort := os.LookupEnv("KUBERNETES_PORT")
	if (hasServiceHost && hasPort) || c.ConfigFile == "" {
		// creates the in-cluster config
		cfg, err := rest.InClusterConfig()
		if err != nil {
			log.Fatalf("get k8s client in cluster config error: %s", err.Error())
		}
		return cfg
	} else {
		cfg, err := clientcmd.BuildConfigFromFlags("", c.ConfigFile)
		if err != nil {
			log.Fatalf("build k8s client config error: %s", err.Error())
		}
		return cfg
	}
}

func MustNewClient(c cfg.KubeConfig) *Client {
	config := GetKubeConfig(c)
	coreClient := kubernetes.NewForConfigOrDie(config)
	dynamicClient := dynamic.NewForConfigOrDie(config)
	return &Client{
		CoreClient:    coreClient,
		DynamicClient: dynamicClient,
	}
}

func (c *Client) CoordinationV1Client() coordinationv1.CoordinationV1Interface {
	return c.CoreClient.CoordinationV1()
}
