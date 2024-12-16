package k8s

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/scitix/sichek/config"

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
	k8sClientOnce.Do(func() {
		var cfg *rest.Config
		var err error

		_, hasServiceHost := os.LookupEnv("KUBERNETES_SERVICE_HOST")
		_, hasPort := os.LookupEnv("KUBERNETES_PORT")
		if hasServiceHost && hasPort {
			cfg, err = rest.InClusterConfig()
			if err != nil {
				logrus.Fatalf("build in-cluster kubeconfig failed: %v", err)
				panic(err)
			}
		} else {
			cfg, err = clientcmd.BuildConfigFromFlags("", config.KubeConfigPath)
			if err != nil {
				logrus.Warnf("get kubeconfig faield, build in-cluster config: %v", err)
				panic(err)
			}
		}

		cli, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			panic(err)
		}

		k8sClient = &K8sClient{
			kubeconfig: config.KubeConfigPath,
			client:     cli,
		}
	})
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
