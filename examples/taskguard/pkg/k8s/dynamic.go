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

	trainingv1 "github.com/kubeflow/training-operator/pkg/apis/kubeflow.org/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CRDs
const (
	ResourcePytorchJob string = "pytorchjobs"
)

var PytorchJobGVR = schema.GroupVersionResource{
	Group:    trainingv1.GroupVersion.Group,
	Version:  trainingv1.GroupVersion.Version,
	Resource: ResourcePytorchJob,
}

func (c *Client) CreatePytorchJob(ctx context.Context, namespace string, pj *trainingv1.PyTorchJob) (*unstructured.Unstructured, error) {
	unstructuredPj := new(unstructured.Unstructured)
	unstructuredPjObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pj)
	if err != nil {
		return nil, err
	}
	unstructuredPj.Object = unstructuredPjObj

	utd, err := c.DynamicClient.Resource(PytorchJobGVR).Namespace(namespace).Create(ctx, unstructuredPj, metav1.CreateOptions{})
	if err != nil {
		// job exists
		if k8serr.IsAlreadyExists(err) {
			return nil, nil
		}
		return nil, err
	}
	return utd, nil
}

func (c *Client) DeletePytorchJob(ctx context.Context, namespace, jobName string) error {
	err := c.DynamicClient.Resource(PytorchJobGVR).Namespace(namespace).Delete(ctx, jobName, metav1.DeleteOptions{})
	if err != nil {
		// pytorchjob has been deleted, pass
		if k8serr.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

// func (c *Client) StopPytorchJob(ctx context.Context, namespace string, cfg *rest.Config, task *models.Task, cluster string) error {

// 	// 模仿task-logic里的ListPodsByLabels，得到podsName
// 	lbs := labels.Set{
// 		"training.kubeflow.org/job-name": task.Name,
// 	}
// 	pods, err := c.ListPodsByLabels(ctx, namespace, lbs)
// 	if err != nil {
// 		return fmt.Errorf("failed to list pods, err: %v", err)
// 	}

// 	// TESTENV
// 	if cluster == "odysseus" && len(pods) == 0 {
// 		if task.Type == models.TaskTypePytorchJob {
// 			lbs = labels.Set{
// 				"job-name": task.Name,
// 			}
// 		}
// 		pods, err = c.ListPodsByLabels(ctx, namespace, lbs)
// 		if err != nil {
// 			return fmt.Errorf("failed to list pods, err: %v", err)
// 		}
// 		if len(pods) == 0 {
// 			return fmt.Errorf("pods not found, ns: %s, name: %s", namespace, task.Name)
// 		}
// 	}
// 	if len(pods) == 0 {
// 		return fmt.Errorf("pods not found, ns: %s, name: %s", namespace, task.Name)
// 	}

// 	// pytorch当worker没起来只有master起来的时候，也是running状态，因此需要跳过一些没准备好的pod的报错，把剩余pods给Stop
// 	err_str := ""
// 	for _, pod := range pods {
// 		podName := pod.Name
// 		containerName := pod.Spec.Containers[0].Name

// 		err = c.StopPodContainer(ctx, namespace, podName, containerName, cfg)
// 		if err != nil {
// 			// 由于pytorchjob可能有多个pod，因此找不到Pod时只打印日志，不报错
// 			if k8serr.IsNotFound(err) {
// 				log.Printf("skip error of 'pod not found', namespace: %s, podName: %s, containerName: %s", namespace, podName, containerName)
// 				continue
// 			}
// 			err_str += err.Error() + "  "
// 		}
// 	}

// 	if err_str != "" {
// 		return errors.New(err_str)
// 	}
// 	return nil
// }

func (c *Client) GetPytorchJobByName(ctx context.Context, namespace, name string) (*trainingv1.PyTorchJob, error) {
	utd, err := c.DynamicClient.Resource(PytorchJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// if not found, give an empty resp
		if k8serr.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	pj := new(trainingv1.PyTorchJob)
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(utd.UnstructuredContent(), pj)
	if err != nil {
		return pj, err
	}
	return pj, nil
}

func (c *Client) ListPytorchJobByLabels(ctx context.Context, namespace string, labelSet labels.Set) ([]*trainingv1.PyTorchJob, error) {
	labelSelector := labels.SelectorFromSet(labelSet)
	return c.ListPytorchJobBySelector(ctx, namespace, labelSelector)
}

func (c *Client) ListPytorchJobBySelector(ctx context.Context, namespace string, selector labels.Selector) ([]*trainingv1.PyTorchJob, error) {
	pytorchJobs, err := c.DynamicClient.Resource(PytorchJobGVR).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}

	jobs := make([]*trainingv1.PyTorchJob, len(pytorchJobs.Items))
	for i, job := range pytorchJobs.Items {
		pj := new(trainingv1.PyTorchJob)
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(job.UnstructuredContent(), pj)
		if err != nil {
			return jobs, err
		}
		jobs[i] = pj
	}
	return jobs, nil
}
