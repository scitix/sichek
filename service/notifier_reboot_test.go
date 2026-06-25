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
package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const testAnnoKey = "scitix.ai/sichek"

// fakeNodeClient is an in-memory stand-in for the K8s node-annotation client so
// the notifier can be exercised without a cluster.
type fakeNodeClient struct {
	anno      map[string]string
	getErr    error
	updateErr error
}

func (f *fakeNodeClient) GetCurrNode(ctx context.Context) (*v1.Node, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	a := f.anno
	if a == nil {
		a = map[string]string{}
	}
	return &v1.Node{ObjectMeta: metav1.ObjectMeta{Annotations: a}}, nil
}

func (f *fakeNodeClient) UpdateNodeAnnotation(ctx context.Context, anno map[string]string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	if f.anno == nil {
		f.anno = map[string]string{}
	}
	for k, v := range anno {
		f.anno[k] = v
	}
	return nil
}

func newTestNotifier(fake *fakeNodeClient) *notifier {
	return &notifier{k8sClient: fake, annoKey: testAnnoKey}
}

func ibResult(status, level string) *common.Result {
	return &common.Result{
		Item:   consts.ComponentNameInfiniband,
		Status: status,
		Level:  level,
		Checkers: []*common.CheckerResult{
			{Name: "x", Status: status, Level: level, ErrorName: "IBErr", Device: "mlx5_0"},
		},
	}
}

func writtenAnno(t *testing.T, fake *fakeNodeClient) *nodeAnnotation {
	t.Helper()
	var a nodeAnnotation
	require.NoError(t, json.Unmarshal([]byte(fake.anno[testAnnoKey]), &a))
	return &a
}

// ResetNodeAnnotation must wipe stale issues left over from before a reboot.
func TestResetNodeAnnotationClearsStale(t *testing.T) {
	seed := &nodeAnnotation{
		Infiniband: map[string][]*annotation{consts.LevelCritical: {{ErrorName: "OldErr", Device: "mlx5_0"}}},
		NVIDIA:     map[string][]*annotation{consts.LevelFatal: {{ErrorName: "Xid", Device: "gpu0"}}},
	}
	seedStr, err := seed.JSON()
	require.NoError(t, err)
	fake := &fakeNodeClient{anno: map[string]string{testAnnoKey: seedStr}}
	n := newTestNotifier(fake)

	anno, err := n.ResetNodeAnnotation(context.Background())
	require.NoError(t, err)
	require.NotNil(t, anno)
	assert.Empty(t, anno.Infiniband)
	assert.Empty(t, anno.NVIDIA)

	w := writtenAnno(t, fake)
	assert.Empty(t, w.Infiniband)
	assert.Empty(t, w.NVIDIA)
}

// With no K8s client, reset is a no-op (non-K8s environment).
func TestResetNodeAnnotationNoK8s(t *testing.T) {
	n := &notifier{annoKey: testAnnoKey}
	anno, err := n.ResetNodeAnnotation(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, anno)
}

// An unparseable existing annotation must not permanently block updates: the
// notifier should start from empty and still write a valid annotation.
func TestSetNodeAnnotationParseResilience(t *testing.T) {
	fake := &fakeNodeClient{anno: map[string]string{testAnnoKey: "{not valid json"}}
	n := newTestNotifier(fake)

	_, err := n.SetNodeAnnotation(context.Background(), ibResult(consts.StatusNormal, consts.LevelInfo))
	require.NoError(t, err)

	w := writtenAnno(t, fake)
	assert.Empty(t, w.Infiniband)
}

// A healthy result clears its own component's issues but preserves others.
func TestSetNodeAnnotationClearsHealthyKeepsOthers(t *testing.T) {
	seed := &nodeAnnotation{
		Infiniband: map[string][]*annotation{consts.LevelCritical: {{ErrorName: "IBErr", Device: "mlx5_0"}}},
		NVIDIA:     map[string][]*annotation{consts.LevelFatal: {{ErrorName: "Xid", Device: "gpu0"}}},
	}
	seedStr, _ := seed.JSON()
	fake := &fakeNodeClient{anno: map[string]string{testAnnoKey: seedStr}}
	n := newTestNotifier(fake)

	_, err := n.SetNodeAnnotation(context.Background(), ibResult(consts.StatusNormal, consts.LevelInfo))
	require.NoError(t, err)

	w := writtenAnno(t, fake)
	assert.Empty(t, w.Infiniband, "healthy infiniband result should clear its stale issues")
	assert.NotEmpty(t, w.NVIDIA, "other component's issues must be preserved")
}

func TestParseAnnotationOrEmpty(t *testing.T) {
	assert.Empty(t, parseAnnotationOrEmpty("").Infiniband)            // empty string
	assert.Empty(t, parseAnnotationOrEmpty("{bad").Infiniband)        // invalid json
	good := &nodeAnnotation{Infiniband: map[string][]*annotation{consts.LevelCritical: {{ErrorName: "E"}}}}
	s, _ := good.JSON()
	assert.NotEmpty(t, parseAnnotationOrEmpty(s).Infiniband) // valid json round-trips
}
