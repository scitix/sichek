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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func serveScript(t *testing.T, body string, status int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestParseKV(t *testing.T) {
	m := parseKV("a=1\n\nb=x=y\nnoeq\nc=NA\n")
	assert.Equal(t, "1", m["a"])
	assert.Equal(t, "x=y", m["b"]) // split on first = only
	assert.Equal(t, "NA", m["c"])
	_, ok := m["noeq"]
	assert.False(t, ok) // lines without = are skipped
	assert.Len(t, m, 3)
}

func TestCollectOK(t *testing.T) {
	url := serveScript(t, "#!/usr/bin/env bash\necho 'kernel.release=5.15'\necho 'x.y=1'\n", 0)
	res := Collect(context.Background(), "os_config", url, 10*time.Second)
	assert.Equal(t, StatusOK, res.Status)
	assert.Equal(t, url, res.Source)
	assert.Equal(t, "5.15", res.Raw["kernel.release"])
	assert.Equal(t, 2, res.KeyCount)
	assert.Empty(t, res.Error)
}

func TestCollectScriptNonZeroExit(t *testing.T) {
	url := serveScript(t, "#!/usr/bin/env bash\necho 'oops' >&2\nexit 1\n", 0)
	res := Collect(context.Background(), "os_config", url, 10*time.Second)
	assert.Equal(t, StatusFailed, res.Status)
	assert.NotEmpty(t, res.Error)
	assert.Empty(t, res.Raw)
}

func TestCollectDownloadFailure(t *testing.T) {
	url := serveScript(t, "nope", http.StatusServiceUnavailable)
	res := Collect(context.Background(), "os_config", url, 10*time.Second)
	assert.Equal(t, StatusFailed, res.Status)
	assert.Contains(t, res.Error, "download")
}

func TestCollectTimeout(t *testing.T) {
	url := serveScript(t, "#!/usr/bin/env bash\nsleep 5\necho 'a=1'\n", 0)
	res := Collect(context.Background(), "os_config", url, 200*time.Millisecond)
	assert.Equal(t, StatusFailed, res.Status)
	assert.Contains(t, res.Error, "timeout")
}

func TestCollectRejectsInsecureScheme(t *testing.T) {
	res := Collect(context.Background(), "os_config", "http://example.com/collect.sh", 10*time.Second)
	assert.Equal(t, StatusFailed, res.Status)
	assert.Contains(t, res.Error, "https")
	assert.Empty(t, res.Raw)
}
