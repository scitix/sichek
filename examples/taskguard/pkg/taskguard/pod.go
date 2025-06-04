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

	trainingv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type PodInformerGroup struct {
	InformerFactory informers.SharedInformerFactory
	Informer        cache.SharedIndexInformer
	Lister          corev1listers.PodLister
}

func NewPodInformerGroup(client *kubernetes.Clientset, resyncPeriod time.Duration) *PodInformerGroup {
	podInformerFactory := informers.NewSharedInformerFactory(
		client,
		resyncPeriod,
	)
	podInformer := podInformerFactory.Core().V1().Pods().Informer()
	podLister := podInformerFactory.Core().V1().Pods().Lister()
	return &PodInformerGroup{
		InformerFactory: podInformerFactory,
		Informer:        podInformer,
		Lister:          podLister,
	}
}

func (g *PodInformerGroup) MustStart(ctx context.Context) {
	g.InformerFactory.Start(ctx.Done())
	for informerType, ok := range g.InformerFactory.WaitForCacheSync(ctx.Done()) {
		if !ok {
			log.Fatalf("timed out waiting for pod informer factory cache sync, informer type is %v", informerType)
		}
	}
	logx.Info("pod informer factory cache sync finished")
}

func (g *PodInformerGroup) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return g.Informer.AddEventHandler(handler)
}

func (g *PodInformerGroup) ListPodsByLabels(namespace string, lbs labels.Set) ([]*corev1.Pod, error) {
	return g.Lister.Pods(namespace).List(lbs.AsSelector())
}

func (g *PodInformerGroup) ListPytorchJobPods(namespace, name string) ([]*corev1.Pod, error) {
	lbs := labels.Set{
		trainingv1.JobNameLabel: name,
	}
	return g.ListPodsByLabels(namespace, lbs)
}
