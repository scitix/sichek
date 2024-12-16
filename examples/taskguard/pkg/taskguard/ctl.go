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
package taskguard

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/scitix/taskguard/pkg/cfg"
	"github.com/scitix/taskguard/pkg/k8s"
	"github.com/scitix/taskguard/pkg/svc"

	trainingv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/zeromicro/go-zero/core/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	discovery "k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	LabelRetryIndex string = "retry-index"
)

type Controller struct {
	svcConfig              cfg.Config
	config                 cfg.FaultToleranceConfig
	k8sClient              *k8s.Client
	nodeInformerFactory    informers.SharedInformerFactory
	nodeInformer           cache.SharedInformer
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory
	pjInformer             cache.SharedIndexInformer
}

func MustNewController(svcCtx *svc.ServiceContext) *Controller {
	config := k8s.GetKubeConfig(svcCtx.Config.KubeConfig)
	faultToleranceConfig := svcCtx.Config.FaultToleranceConfig
	k8sClient := svcCtx.K8SClient
	coreClient := k8sClient.CoreClient
	dynamicClient := k8sClient.DynamicClient

	// informers
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		log.Fatalf("get discovery client error: %s", err.Error())

	}
	_, apiResourceList, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		log.Fatalf("get api resources error: %s", err.Error())

	}
	existPytorchJob := false
	for _, list := range apiResourceList {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			log.Fatalf("parse Group Version error: %s", err.Error())
		}
		for _, resource := range list.APIResources {
			if resource.Name == k8s.PytorchJobGVR.Resource {
				if gv.Group == k8s.PytorchJobGVR.Group && gv.Version == k8s.PytorchJobGVR.Version {
					existPytorchJob = true
				} else {
					log.Fatalf("pytorchjob resource group not match, found group: %s, version: %s", gv.Group, gv.Version)
				}
			}
		}
	}
	if !existPytorchJob {
		log.Fatalf("pytorchjob resource not found")
	}

	// node informers
	nodeInformerFactory := informers.NewSharedInformerFactory(
		coreClient,
		20*time.Minute,
	)
	nodeInformer := nodeInformerFactory.Core().V1().Nodes().Informer()

	// pytorchjob informers
	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(
		dynamicClient,
		20*time.Minute,
	)
	pjInformer := dynamicInformerFactory.ForResource(k8s.PytorchJobGVR).Informer()

	return &Controller{
		svcConfig:              svcCtx.Config,
		config:                 faultToleranceConfig,
		k8sClient:              k8sClient,
		nodeInformerFactory:    nodeInformerFactory,
		nodeInformer:           nodeInformer,
		dynamicInformerFactory: dynamicInformerFactory,
		pjInformer:             pjInformer,
	}
}

func (c *Controller) RunOrDie(ctx context.Context) {
	var err error

	c.nodeInformerFactory.Start(ctx.Done())
	for informerType, ok := range c.nodeInformerFactory.WaitForCacheSync(ctx.Done()) {
		if !ok {
			log.Fatalf("timed out waiting for node informer factory cache sync, informer type is %v", informerType)
		}
	}
	logx.Info("node informer factory cache sync finished")

	c.dynamicInformerFactory.Start(ctx.Done())
	for informerType, ok := range c.dynamicInformerFactory.WaitForCacheSync(ctx.Done()) {
		if !ok {
			log.Fatalf("timed out waiting for dynamic informer factory cache sync, informer type is %v", informerType)
		}
	}
	logx.Info("dynamic informer factory cache sync finished")

	if c.config.EnableTaskGuardLabel != "" {
		_, err = c.pjInformer.AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: func(obj any) bool {
				pj, ok := obj.(*trainingv1.PyTorchJob)
				if !ok {
					return false
				}
				return pj.Labels[c.config.EnableTaskGuardLabel] == "true"
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    c.addPytorchJob,
				UpdateFunc: c.updatePytorchJob,
			},
		})
	} else {
		_, err = c.pjInformer.AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc:    c.addPytorchJob,
				UpdateFunc: c.updatePytorchJob,
			},
		)
	}
	if err != nil {
		panic(fmt.Sprintf("error to add event handler for pytorchjob informer, error: %v", err))
	}

	checkPeriod := c.config.CheckStatusPeriod
	go c.checkTaskStatusPeriodically(ctx, checkPeriod)
	<-ctx.Done()
}

func (c *Controller) checkTaskStatusPeriodically(ctx context.Context, checkPeriod time.Duration) {
	logx.Info("Start auto fault tolerance checking")
	wait.Until(func() {
		logx.Info("Auto fault tolerance checking")

		// check pytorchjobs with taskguard label
		lbs := labels.Set{}
		if len(c.config.EnableTaskGuardLabel) != 0 {
			logx.Infof("enable task guard label %s", c.config.EnableTaskGuardLabel)
			lbs = labels.Set{
				c.config.EnableTaskGuardLabel: "true",
			}
		}

		pytorchJobs, err := c.k8sClient.ListPytorchJobByLabels(ctx, metav1.NamespaceAll, lbs)
		if err != nil {
			logx.Errorf("failed to list pytorchjobs, err: %s", err.Error())
		}
		for _, job := range pytorchJobs {
			jobNamespace := job.Namespace
			jobName := job.Name

			jobConditions := job.Status.Conditions
			jobConditionsLen := len(jobConditions)

			if jobConditionsLen > 0 && jobConditions[jobConditionsLen-1].Type == trainingv1.JobRunning {
				logx.Infof("parse pytorchjob %s, namespace: %s", jobName, jobNamespace)
				jobPods, err := c.getPytorchJobPods(ctx, jobNamespace, jobName)
				if err != nil {
					logx.Errorf("failed to get pytorchjob pods, err: %s", err.Error())
					continue
				}
				for _, pod := range jobPods {
					if c.isTaskPodHangFromLog(ctx, jobNamespace, pod.Name) || c.isTaskPodHangFromSiChek(pod.Spec.NodeName, pod.Name) {
						logx.Infof("pod %s in task pytorchjob %s is hang", pod.Name, jobName)
						_, err := c.resubmitPytorchJob(ctx, job, jobPods, false)
						if err != nil {
							logx.Errorf("failed to resubmit pytorchjob, ns: %s, name: %s", jobNamespace, jobName)
						}
						logx.Infof("task pytorchjob %s resubmit succeed", jobName)
						break
					}
				}
			}
		}
	}, checkPeriod, ctx.Done())
}
