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
package sysinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scitix/sichek/components/common"
	"github.com/scitix/sichek/components/sysinfo/collector"
	"github.com/scitix/sichek/components/sysinfo/config"
	"github.com/scitix/sichek/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serveScript(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func bp(b bool) *bool { return &b }

func newTestComponent(cfg *config.SysinfoConfig) *component {
	ctx, cancel := context.WithCancel(context.Background())
	return &component{
		ctx:           ctx,
		cancel:        cancel,
		componentName: consts.ComponentNameSysinfo,
		cfg:           &config.SysinfoUserConfig{Sysinfo: cfg},
		outputs:       map[string]*collector.SourceResult{},
		resultCh:      make(chan *common.Result),
	}
}

func TestHealthCheckRunsAllSources(t *testing.T) {
	u1 := serveScript(t, "echo 'a=1'\n")
	u2 := serveScript(t, "echo 'b=2'\n")
	c := newTestComponent(&config.SysinfoConfig{
		Enable:  bp(true),
		Timeout: common.Duration{Duration: 10 * time.Second},
		Sources: []config.SourceSpec{{Name: "s1", URL: u1}, {Name: "s2", URL: u2}},
	})
	_, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	info, err := c.LastInfo()
	require.NoError(t, err)
	out := info.(*SysinfoOutput)
	assert.Equal(t, collector.StatusOK, out.Sources["s1"].Status)
	assert.Equal(t, "1", out.Sources["s1"].Raw["a"])
	assert.Equal(t, "2", out.Sources["s2"].Raw["b"])
}

func TestOneSourceFailureIsolated(t *testing.T) {
	good := serveScript(t, "echo 'a=1'\n")
	c := newTestComponent(&config.SysinfoConfig{
		Enable:  bp(true),
		Timeout: common.Duration{Duration: 10 * time.Second},
		Sources: []config.SourceSpec{
			{Name: "good", URL: good},
			{Name: "bad", URL: "http://127.0.0.1:1/nope.sh"},
		},
	})
	_, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	out := mustInfo(t, c)
	assert.Equal(t, collector.StatusOK, out.Sources["good"].Status)
	assert.Equal(t, collector.StatusFailed, out.Sources["bad"].Status)
}

func TestStartEmitsAndStops(t *testing.T) {
	u1 := serveScript(t, "echo 'a=1'\n")
	c := newTestComponent(&config.SysinfoConfig{
		Enable:        bp(true),
		Timeout:       common.Duration{Duration: 10 * time.Second},
		QueryInterval: common.Duration{Duration: time.Hour}, // ticker won't fire during test
		Sources:       []config.SourceSpec{{Name: "s1", URL: u1}},
	})
	ch := c.Start()
	select {
	case res := <-ch: // immediate run emits a benign result
		assert.Equal(t, consts.StatusNormal, res.Status)
	case <-time.After(3 * time.Second):
		t.Fatal("no result emitted on startup")
	}
	out := mustInfo(t, c)
	assert.Equal(t, "1", out.Sources["s1"].Raw["a"])
	require.NoError(t, c.Stop())
}

func TestDisabledSkipsCollection(t *testing.T) {
	c := newTestComponent(&config.SysinfoConfig{
		Enable:  bp(false),
		Timeout: common.Duration{Duration: time.Second},
		Sources: []config.SourceSpec{{Name: "s1", URL: serveScript(t, "a=1\n")}},
	})
	_, err := c.HealthCheck(context.Background())
	require.NoError(t, err)
	assert.Empty(t, mustInfo(t, c).Sources)
}

func mustInfo(t *testing.T, c *component) *SysinfoOutput {
	t.Helper()
	info, err := c.LastInfo()
	require.NoError(t, err)
	return info.(*SysinfoOutput)
}
