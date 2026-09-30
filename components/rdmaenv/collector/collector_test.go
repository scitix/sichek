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

package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectSuccess(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "metrics_sample.txt"))
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewCollector(srv.URL, 5*time.Second, "rdma_env_pre_")
	info, err := c.Collect(context.Background())
	require.NoError(t, err)
	assert.True(t, info.Available)
	assert.Empty(t, info.Error)
	assert.NotEmpty(t, info.Series)
	assert.Equal(t, "drift", info.Summary.HostCompliance)
	assert.Len(t, info.Summary.Knobs, 4)
}

func TestCollectConnRefusedDegrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // now nothing is listening

	c := NewCollector(url, 1*time.Second, "rdma_env_pre_")
	info, err := c.Collect(context.Background())
	require.NoError(t, err) // failure must NOT be a component fault
	assert.False(t, info.Available)
	assert.NotEmpty(t, info.Error)
	assert.Empty(t, info.Series)
	assert.Equal(t, url, info.Endpoint)
}

func TestCollectNon2xxDegrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewCollector(srv.URL, 1*time.Second, "rdma_env_pre_")
	info, err := c.Collect(context.Background())
	require.NoError(t, err)
	assert.False(t, info.Available)
	assert.NotEmpty(t, info.Error)
}
