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
	"log"
	"time"

	"github.com/scitix/taskguard/pkg/cfg"
	"github.com/scitix/taskguard/pkg/k8s"
	"github.com/scitix/taskguard/pkg/svc"

	trainingv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/zeromicro/go-zero/core/logx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
)

type Controller struct {
	svcConfig cfg.Config
	config    cfg.FaultToleranceConfig
	k8sClient *k8s.Client
	pod       *PodInformerGroup
	node      *NodeInformerGroup
	pj        *PytorchJobInformerGroup
}

func MustNewController(svcCtx *svc.ServiceContext) *Controller {
	faultToleranceConfig := svcCtx.Config.FaultToleranceConfig
	coreClient := svcCtx.K8SClient.CoreClient
	trainV1Client := svcCtx.K8SClient.TrainingV1Client

	podInformerGroup := NewPodInformerGroup(coreClient, faultToleranceConfig.ResyncPeriod)
	nodeInformerGroup := NewNodeInformerGroup(coreClient, faultToleranceConfig.ResyncPeriod)
	pjInformerGroup := NewPytorchJobInformerGroup(trainV1Client, faultToleranceConfig.ResyncPeriod)
	return &Controller{
		svcConfig: svcCtx.Config,
		config:    faultToleranceConfig,
		k8sClient: svcCtx.K8SClient,
		pod:       podInformerGroup,
		node:      nodeInformerGroup,
		pj:        pjInformerGroup,
	}
}

func (c *Controller) RunOrDie(ctx context.Context) {
	var err error

	c.pod.MustStart(ctx)
	c.node.MustStart(ctx)
	c.pj.MustStart(ctx)

	if c.config.EnableTaskGuardLabel != "" {
		c.pj.AddEventHandler(cache.FilteringResourceEventHandler{
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
		_, err = c.pj.AddEventHandler(
			cache.ResourceEventHandlerFuncs{
				AddFunc:    c.addPytorchJob,
				UpdateFunc: c.updatePytorchJob,
			},
		)
	}
	if err != nil {
		log.Fatalf("error to add event handler for pytorchjob informer, error: %v", err)
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
			logx.Infof("enable taskguard label %s", c.config.EnableTaskGuardLabel)
			lbs = labels.Set{
				c.config.EnableTaskGuardLabel: "true",
			}
		}

		jobs, err := c.pj.ListPytorchJobsByLabels(metav1.NamespaceAll, lbs)
		if err != nil {
			logx.Errorf("failed to list pytorchjobs, err: %s", err.Error())
		}
		for _, job := range jobs {
			jobNamespace := job.Namespace
			jobName := job.Name

			jobConditions := job.Status.Conditions
			jobConditionsLen := len(jobConditions)

			if jobConditionsLen > 0 && jobConditions[jobConditionsLen-1].Type == trainingv1.JobRunning {
				logx.Infof("parse pytorchjob %s, namespace: %s", jobName, jobNamespace)
				jobPods, err := c.pod.ListPytorchJobPods(jobNamespace, jobName)
				if err != nil {
					logx.Errorf("failed to get pytorchjob pods, err: %s", err.Error())
					continue
				}
				for _, pod := range jobPods {
					if c.isTaskPodHangFromSiChek(pod.Spec.NodeName, pod.Name) {
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
