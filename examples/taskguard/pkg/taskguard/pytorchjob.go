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
	"strconv"
	"time"

	trainingv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	trainingclient "github.com/kubeflow/training-operator/pkg/client/clientset/versioned"
	traininginformers "github.com/kubeflow/training-operator/pkg/client/informers/externalversions"
	traininglisters "github.com/kubeflow/training-operator/pkg/client/listers/kubeflow.org/v1"
	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type PytorchJobInformerGroup struct {
	InformerFactory traininginformers.SharedInformerFactory
	Informer        cache.SharedIndexInformer
	Lister          traininglisters.PyTorchJobLister
}

func NewPytorchJobInformerGroup(client *trainingclient.Clientset, resyncPeriod time.Duration) *PytorchJobInformerGroup {
	pjInformerFactory := traininginformers.NewSharedInformerFactory(
		client,
		resyncPeriod,
	)
	pjInformer := pjInformerFactory.Kubeflow().V1().PyTorchJobs().Informer()
	pjLister := pjInformerFactory.Kubeflow().V1().PyTorchJobs().Lister()
	return &PytorchJobInformerGroup{
		InformerFactory: pjInformerFactory,
		Informer:        pjInformer,
		Lister:          pjLister,
	}
}

func (g *PytorchJobInformerGroup) MustStart(ctx context.Context) {
	g.InformerFactory.Start(ctx.Done())
	for informerType, ok := range g.InformerFactory.WaitForCacheSync(ctx.Done()) {
		if !ok {
			log.Fatalf("timed out waiting for pytorchjob informer factory cache sync, informer type is %v", informerType)
		}
	}
	logx.Info("pytorchjob informer factory cache sync finished")
}

func (g *PytorchJobInformerGroup) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return g.Informer.AddEventHandler(handler)
}

func (g *PytorchJobInformerGroup) ListPytorchJobsByLabels(namespace string, lbs labels.Set) ([]*trainingv1.PyTorchJob, error) {
	return g.Lister.PyTorchJobs(namespace).List(lbs.AsSelector())
}

func (c *Controller) addPytorchJob(obj any) {
	pj, ok := obj.(*trainingv1.PyTorchJob)
	if !ok {
		return
	}

	// only deal the task with failed status
	pjConditions := pj.Status.Conditions
	pjConditionsLen := len(pjConditions)
	if pjConditionsLen == 0 {
		return
	}
	jobStatusObj := pjConditions[pjConditionsLen-1]
	if jobStatusObj.Type != trainingv1.JobFailed {
		return
	}

	ctx := context.Background()
	pods, err := c.pod.ListPytorchJobPods(pj.Namespace, pj.Name)
	if err != nil {
		logx.Errorf("failed to get pytorchjob pods, err: %s", err.Error())
		return
	}
	for _, pod := range pods {
		if !c.isTaskPodHealthy(pod.Spec.NodeName, pod.Name) {
			logx.Infof("retry to resubmit pytorchjob since node unhealthy, namespace: %s, name: %s", pj.Namespace, pj.Name)
			_, err := c.resubmitPytorchJob(ctx, pj, pods, false)
			if err != nil {
				logx.Errorf("failed to resubmit pytorchjob, namespace: %s, name: %s, err: %s", pj.Namespace, pj.Name, err)
			} else {
				logx.Infof("task pytorchjob %s resubmit succeed", pj.Name)
			}
			break
		}
	}
}

func (c *Controller) updatePytorchJob(oldObj, newObj any) {
	oldPj, ok := oldObj.(*trainingv1.PyTorchJob)
	if !ok {
		return
	}
	newPj, ok := newObj.(*trainingv1.PyTorchJob)
	if !ok {
		return
	}

	// early stop
	if oldPj.ResourceVersion == newPj.ResourceVersion {
		return
	}
	newPjConditions := newPj.Status.Conditions
	newPjConditionsLen := len(newPjConditions)
	if newPjConditionsLen == 0 {
		return
	}
	oldPjConditions := oldPj.Status.Conditions
	oldPjConditionsLen := len(oldPjConditions)
	if oldPjConditionsLen > 0 &&
		oldPjConditions[oldPjConditionsLen-1].Type == newPjConditions[newPjConditionsLen-1].Type &&
		oldPjConditions[oldPjConditionsLen-1].Message == newPjConditions[newPjConditionsLen-1].Message {
		return
	}

	jobStatusObj := newPjConditions[newPjConditionsLen-1]
	if jobStatusObj.Type == trainingv1.JobFailed {
		logx.Infof("failed pytorchjob %s, namespace: %s", newPj.Name, newPj.Namespace)
		pods, err := c.pod.ListPytorchJobPods(newPj.Namespace, newPj.Name)
		if err != nil {
			logx.Errorf("failed to get pytorchjob pods, err: %s", err.Error())
			return
		}
		for _, pod := range pods {
			podHealthy := c.isTaskPodHealthy(pod.Spec.NodeName, pod.Name)
			if !podHealthy {
				logx.Infof("retry to resubmit pytorchjob since unhealthy, namespace: %s, name: %s", newPj.Namespace, newPj.Name)
				_, err := c.resubmitPytorchJob(context.Background(), newPj, pods, true)
				if err != nil {
					logx.Errorf("failed to resubmit pytorchjob, namespace: %s, name: %s, err: %s", newPj.Namespace, newPj.Name, err)
				} else {
					logx.Infof("task pytorchjob %s resubmit succeed", newPj.Name)
				}
				break
			}
		}
	}
}

func (c *Controller) resubmitPytorchJob(ctx context.Context, pj *trainingv1.PyTorchJob, pods []*corev1.Pod, hasFailed bool) (bool, error) {
	var err error
	var retryIndex int

	if retryIndexStr, ok := pj.Labels[LabelRetryIndex]; ok {
		retryIndex, err = strconv.Atoi(retryIndexStr)
		if err != nil {
			logx.Errorf("failed to get retry index, err: %s", err.Error())
			return false, err
		}
	}

	if retryIndex >= (c.config.MaxRetryCount - 1) {
		return false, fmt.Errorf("skip retry for pytorchjob since max retry count")
	}

	// kill pod if not failed
	if !hasFailed {
		err = c.killPytorchJob(ctx, pj, pods)
		if err != nil {
			logx.Errorf("failed to kill pytorchjob, err: %s", err.Error())
			return false, err
		}
	}

	// change task retry index and create a new task
	pjCopy := (*pj).DeepCopy()
	newPj := &trainingv1.PyTorchJob{}
	newPj.APIVersion = pj.APIVersion
	newPj.Kind = pj.Kind
	if retryIndex == 0 {
		newPj.Name = fmt.Sprintf("%s-%d", pj.Name, retryIndex+1)
	} else {
		idx := len(pj.Name) - len(fmt.Sprintf("-%d", retryIndex))
		newPj.Name = fmt.Sprintf("%s-%d", pj.Name[0:idx], retryIndex+1)
	}
	newPj.GenerateName = pj.GenerateName
	newPj.Namespace = pj.Namespace
	if pj.Labels == nil {
		newPj.Labels = make(map[string]string)
	} else {
		newPj.Labels = pjCopy.Labels
	}
	newPj.Labels[LabelRetryIndex] = strconv.Itoa(retryIndex + 1)
	newPj.Annotations = pjCopy.Annotations
	newPj.Spec = *pjCopy.Spec.DeepCopy()

	_, err = c.k8sClient.CreatePytorchJob(ctx, pj.Namespace, newPj)
	if err != nil {
		logx.Errorf("failed to create pytorchjob, namespace: %s, name: %s, err: %s", pj.Namespace, newPj.Name, err.Error())
		return false, err
	}

	return true, nil
}

// make task failed to delete one worker pod
func (c *Controller) killPytorchJob(ctx context.Context, pj *trainingv1.PyTorchJob, pods []*corev1.Pod) error {
	for _, pod := range pods {
		if pod.Labels[trainingv1.ReplicaTypeLabel] != PytorchJobReplicaTypeWorker {
			continue
		}
		err := c.k8sClient.DeletePod(ctx, pj.Namespace, pod.Name)
		if err != nil {
			logx.Errorf("failed to delete pod, err: %s", err.Error())
			return err
		}
		break
	}
	return nil
}
