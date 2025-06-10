/*
Copyright 2025 The Scitix Authors.

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
package taskguard

import (
	"context"
	"log"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type NodeInformerGroup struct {
	InformerFactory informers.SharedInformerFactory
	Informer        cache.SharedIndexInformer
	Lister          corev1listers.NodeLister
}

func NewNodeInformerGroup(client *kubernetes.Clientset, resyncPeriod time.Duration) *NodeInformerGroup {
	nodeInformerFactory := informers.NewSharedInformerFactory(
		client,
		resyncPeriod,
	)
	nodeInformer := nodeInformerFactory.Core().V1().Nodes().Informer()
	nodeLister := nodeInformerFactory.Core().V1().Nodes().Lister()
	return &NodeInformerGroup{
		InformerFactory: nodeInformerFactory,
		Informer:        nodeInformer,
		Lister:          nodeLister,
	}
}

func (g *NodeInformerGroup) MustStart(ctx context.Context) {
	g.InformerFactory.Start(ctx.Done())
	for informerType, ok := range g.InformerFactory.WaitForCacheSync(ctx.Done()) {
		if !ok {
			log.Fatalf("timed out waiting for node informer factory cache sync, informer type is %v", informerType)
		}
	}
	logx.Info("node informer factory cache sync finished")
}

func (g *NodeInformerGroup) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return g.Informer.AddEventHandler(handler)
}

func (g *NodeInformerGroup) GetNodeByName(name string) (*corev1.Node, error) {
	return g.Lister.Get(name)
}
