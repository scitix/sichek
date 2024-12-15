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
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// pods
func (c *Client) CreatePod(ctx context.Context, namespace string, pod *v1.Pod) (*v1.Pod, error) {
	pod, err := c.CoreClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		// pod exists
		if errors.IsAlreadyExists(err) {
			return nil, nil
		}
		return nil, err
	}
	return pod, nil
}

func (c *Client) DeletePod(ctx context.Context, namespace, podName string) error {
	err := c.CoreClient.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		// pod has been deleted, pass
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) StopPod(ctx context.Context, namespace, podName string, cfg *rest.Config) error {
	pod, err := c.GetPodByName(ctx, namespace, podName)
	if err != nil {
		return fmt.Errorf("failed to get pod %s, err: %v", podName, err)
	}
	// pod not found
	if pod == nil {
		// log.Printf("pod not found, ns: %s, name: %s", namespace, podName)
		return fmt.Errorf("pod not found, ns: %s, name: %s", namespace, podName)
	}

	containerName := pod.Spec.Containers[0].Name
	// 由于pod只有一个，因此找不到Pod时直接报错
	err = c.StopPodContainer(ctx, namespace, podName, containerName, cfg)
	// if errors.IsNotFound(err) {
	// 	log.Printf("skip error of 'pod not found', namespace: %s, podName: %s, containerName: %s", namespace, podName, containerName)
	// 	return nil
	// }
	return err
}

func (c *Client) StopPodContainer(ctx context.Context, namespace, podName, containerName string, cfg *rest.Config) error {
	psOutput, err := c.ExecInPod(ctx, namespace, podName, containerName, cfg, []string{"ps", "-ax"})
	if err != nil {
		return err
	}

	pids := c.ParseAndSortPids(psOutput)
	killPids := strings.Join(pids, " ")
	killCommand := []string{"kill", "-9"}
	killCommand = append(killCommand, strings.Fields(killPids)...)

	_, err = c.ExecInPod(ctx, namespace, podName, containerName, cfg, killCommand)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) ExecInPod(ctx context.Context, namespace, podName, containerName string, cfg *rest.Config, cmd []string) (string, error) {
	log.Printf("ExecInPod detail, namespace: %s, podName: %s, containerName: %s, cmd: %s", namespace, podName, containerName, cmd)
	req := c.CoreClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to init executor when exec command: %s in pod %s, ns : %s, err: %v", cmd, podName, namespace, err)
	}

	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil {
		return "", fmt.Errorf("failed to exec command: %s in pod %s, ns : %s, err: %v, stderr: %s", cmd, podName, namespace, err, stderr.String())
	}

	if stderr.Len() > 0 {
		return "", fmt.Errorf("failed to exec command: %s in pod %s, ns : %s, err: %v, stderr: %s", cmd, podName, namespace, err, stderr.String())
	}
	log.Printf("ExecInPod result: %s", stdout.String())
	return stdout.String(), nil
}

func (c *Client) ParseAndSortPids(psOutput string) []string {
	lines := strings.Split(psOutput, "\n")
	lines = lines[1:]

	var nonSleepPids, sleepPids []string
	var pid1 string

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		pid := fields[0]
		command := strings.Join(fields[4:], " ")

		if pid == "1" {
			pid1 = pid
		} else if strings.Contains(command, "ps -ax") {
			continue
		} else if strings.Contains(command, "sleep") {
			sleepPids = append(sleepPids, pid)
		} else {
			nonSleepPids = append(nonSleepPids, pid)
		}
	}

	pids := append(append(nonSleepPids, sleepPids...), pid1)
	return pids
}

func (c *Client) GetPodByName(ctx context.Context, namespace, podName string) (*v1.Pod, error) {
	pod, err := c.CoreClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		// if not found, give an empty resp
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return pod, nil
}

func (c *Client) ListPodsByLabels(ctx context.Context, namespace string, labelSet labels.Set) ([]v1.Pod, error) {
	labelSelector := labels.SelectorFromSet(labelSet)
	pods, err := c.CoreClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

func (c *Client) GetPodLogs(ctx context.Context, namespace, podName, containerName string, tailLines int64) (string, error) {
	req := c.CoreClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
	})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// nodes
func (c *Client) CordonNode(ctx context.Context, nodeName string) error {
	node, err := c.CoreClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		// if not found, give an empty resp
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	node.Spec.Unschedulable = true
	_, err = c.CoreClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// secrets
func (c *Client) GetSecretByName(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	secret, err := c.CoreClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret, nil
}
