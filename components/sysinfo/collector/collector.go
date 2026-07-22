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
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	StatusOK     = "ok"
	StatusFailed = "failed"
)

// SourceResult is one script's collected output; it is JSON-serialized into
// snapshot.json under components.sysinfo.sources.<name>.
type SourceResult struct {
	Raw         map[string]string `json:"raw"`
	Source      string            `json:"source"`
	Status      string            `json:"status"`
	Error       string            `json:"error,omitempty"`
	KeyCount    int               `json:"key_count"`
	CollectedAt time.Time         `json:"collected_at"`
}

// Collect downloads the script at url, executes it with bash under a timeout,
// and parses its key=value stdout. Every failure mode yields Status=failed with
// an Error and an empty Raw — it never returns nil and never panics.
func Collect(ctx context.Context, name, url string, timeout time.Duration) *SourceResult {
	res := &SourceResult{Raw: map[string]string{}, Source: url, Status: StatusOK, CollectedAt: time.Now()}

	if err := validateURL(url); err != nil {
		return fail(res, err.Error())
	}

	body, err := download(ctx, url, timeout)
	if err != nil {
		return fail(res, fmt.Sprintf("download: %v", err))
	}

	tmp, err := os.CreateTemp("", "sichek-"+sanitize(name)+"-*.sh")
	if err != nil {
		return fail(res, fmt.Sprintf("tempfile: %v", err))
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fail(res, fmt.Sprintf("write temp: %v", err))
	}
	tmp.Close()

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "bash", tmpPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if cctx.Err() == context.DeadlineExceeded {
		return fail(res, "timeout")
	}
	if err != nil {
		return fail(res, fmt.Sprintf("exec: %v: %s", err, strings.TrimSpace(stderr.String())))
	}

	res.Raw = parseKV(stdout.String())
	res.KeyCount = len(res.Raw)
	return res
}

func download(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// parseKV splits each non-blank line on the FIRST '='. Lines without '=' are
// skipped. Duplicate keys: last wins.
func parseKV(out string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		i := strings.Index(line, "=")
		if i < 0 {
			continue
		}
		m[line[:i]] = line[i+1:]
	}
	return m
}

func fail(res *SourceResult, msg string) *SourceResult {
	res.Status = StatusFailed
	res.Error = msg
	res.Raw = map[string]string{}
	res.KeyCount = 0
	return res
}

// validateURL refuses to fetch a root-executed script over an insecure scheme.
// https is always allowed; http is allowed only for loopback hosts (tests and
// local mirrors). Everything else is rejected.
func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("refusing to fetch root-executed script over insecure scheme %q (must be https): %s", u.Scheme, raw)
}

func isLoopbackHost(h string) bool {
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func sanitize(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, name)
}
