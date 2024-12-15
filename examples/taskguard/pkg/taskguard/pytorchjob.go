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
	"strconv"

	trainingv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	"github.com/zeromicro/go-zero/core/logx"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	LabelKubeflowJobName    string = "training.kubeflow.org/job-name"
	LabelKubeflowJobPodType string = "training.kubeflow.org/replica-type"
	LabelPytorchJobName     string = "job-name"
)

func (c *Controller) addPytorchJob(obj any) {
	utd, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	pj := new(trainingv1.PyTorchJob)
	runtime.DefaultUnstructuredConverter.FromUnstructured(utd.UnstructuredContent(), pj)

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
	pods, err := c.getPytorchJobPods(ctx, pj.Namespace, pj.Name)
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
	oldUtd, ok := oldObj.(*unstructured.Unstructured)
	if !ok {
		return
	}
	newUtd, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		return
	}

	var oldPytorchJob, newPytorchJob trainingv1.PyTorchJob
	runtime.DefaultUnstructuredConverter.FromUnstructured(oldUtd.UnstructuredContent(), &oldPytorchJob)
	runtime.DefaultUnstructuredConverter.FromUnstructured(newUtd.UnstructuredContent(), &newPytorchJob)

	// early stop
	if oldPytorchJob.ResourceVersion == newPytorchJob.ResourceVersion {
		return
	}
	newPjConditions := newPytorchJob.Status.Conditions
	newPjConditionsLen := len(newPjConditions)
	if newPjConditionsLen == 0 {
		return
	}
	oldPjConditions := oldPytorchJob.Status.Conditions
	oldPjConditionsLen := len(oldPjConditions)
	if oldPjConditionsLen > 0 && oldPjConditions[oldPjConditionsLen-1].Type == newPjConditions[newPjConditionsLen-1].Type && oldPjConditions[oldPjConditionsLen-1].Message == newPjConditions[newPjConditionsLen-1].Message {
		return
	}

	jobStatusObj := newPjConditions[newPjConditionsLen-1]
	if jobStatusObj.Type == trainingv1.JobFailed {
		ctx := context.Background()
		pods, err := c.getPytorchJobPods(ctx, newPytorchJob.Namespace, newPytorchJob.Name)
		if err != nil {
			logx.Errorf("failed to get pytorchjob pods, err: %s", err.Error())
			return
		}
		for _, pod := range pods {
			nodeHealthy := c.isTaskPodHealthy(pod.Spec.NodeName, pod.Name)
			if !nodeHealthy {
				logx.Infof("retry to resubmit pytorchjob since node unhealthy, namespace: %s, name: %s", newPytorchJob.Namespace, newPytorchJob.Name)
				_, err := c.resubmitPytorchJob(ctx, &newPytorchJob, pods, true)
				if err != nil {
					logx.Errorf("failed to resubmit pytorchjob, namespace: %s, name: %s, err: %s", newPytorchJob.Namespace, newPytorchJob.Name, err)
				} else {
					logx.Infof("task pytorchjob %s resubmit succeed", newPytorchJob.Name)
				}
				break
			}
		}
	}
}

func (c *Controller) resubmitPytorchJob(ctx context.Context, pj *trainingv1.PyTorchJob, pods []v1.Pod, hasFailed bool) (bool, error) {
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
		newPj.Labels = pj.DeepCopy().Labels
	}
	newPj.Labels[LabelRetryIndex] = strconv.Itoa(retryIndex + 1)
	newPj.Annotations = pj.DeepCopy().Annotations
	newPj.Spec = *pj.Spec.DeepCopy()

	_, err = c.k8sClient.CreatePytorchJob(ctx, pj.Namespace, newPj)
	if err != nil {
		logx.Errorf("failed to create pytorchjob, namespace: %s, name: %s, err: %s", pj.Namespace, newPj.Name, err.Error())
		return false, err
	}

	return true, nil
}

// make task failed to delete one worker pod
func (c *Controller) killPytorchJob(ctx context.Context, pj *trainingv1.PyTorchJob, pods []v1.Pod) error {
	for _, pod := range pods {
		if pod.Labels[LabelKubeflowJobPodType] != "worker" {
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

func (c *Controller) getPytorchJobPods(ctx context.Context, jobNamespace, jobName string) ([]v1.Pod, error) {
	lbs := labels.Set{
		LabelKubeflowJobName: jobName,
	}

	return c.k8sClient.ListPodsByLabels(ctx, jobNamespace, lbs)
}
