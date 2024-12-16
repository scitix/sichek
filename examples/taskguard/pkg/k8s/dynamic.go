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
