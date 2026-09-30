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
	"strings"
	"sync"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/consts"
)

// AnnotationStore is an in-process accumulator of detected issues used to feed
// the snapshot when no K8s notifier is available (non-K8s environments).
//
// On K8s nodes the notifier is the source of truth: it reads, accumulates and
// writes the node annotation, and returns the resulting *nodeAnnotation directly,
// so this store is only exercised when notifier == nil. The two paths are
// mutually exclusive and never run concurrently.
type AnnotationStore struct {
	mu   sync.Mutex
	anno *nodeAnnotation
}

// NewAnnotationStore returns an empty accumulator. Off-K8s there is no node
// annotation to seed from, so accumulation starts fresh and is rebuilt as
// checks run.
func NewAnnotationStore() *AnnotationStore {
	return &AnnotationStore{anno: &nodeAnnotation{}}
}

// Apply folds a component result into the accumulated annotation using the same
// rule as the K8s path: a HealthCheckTimeout abnormal result is appended (it must
// not wipe previously recorded real issues), while every other result replaces
// that item's issues. It returns a deep copy of the current accumulated
// annotation so callers can persist it without racing the next Apply.
//
// On error (e.g. an append on an item not tracked by the annotation schema) the
// accumulated state is left unchanged and still returned, so the snapshot keeps
// reflecting the last good annotation.
func (s *AnnotationStore) Apply(result *common.Result) (*nodeAnnotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if isHealthCheckTimeout(result) {
		err = s.anno.AppendFromResult(result)
	} else {
		err = s.anno.ParseFromResult(result)
	}
	return s.anno.deepCopy(), err
}

// isHealthCheckTimeout reports whether result is an abnormal HealthCheckTimeout
// result. Such results must be appended rather than replacing the item's issues,
// otherwise a transient timeout would erase previously detected real problems.
func isHealthCheckTimeout(result *common.Result) bool {
	return result != nil &&
		len(result.Checkers) > 0 &&
		strings.Contains(result.Checkers[0].Name, "HealthCheckTimeout") &&
		result.Status == consts.StatusAbnormal
}
