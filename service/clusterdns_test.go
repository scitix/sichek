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
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProc lays out a minimal /proc with one kubelet process.
func fakeProc(t *testing.T, comm string, args ...string) string {
	t.Helper()
	root := t.TempDir()
	pid := filepath.Join(root, "4242")
	require.NoError(t, os.MkdirAll(pid, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pid, "comm"), []byte(comm+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(pid, "cmdline"), []byte(strings.Join(args, "\x00")+"\x00"), 0o644))
	// A non-pid entry and a non-kubelet process must be ignored.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "self"), 0o755))
	other := filepath.Join(root, "1")
	require.NoError(t, os.MkdirAll(other, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(other, "comm"), []byte("systemd\n"), 0o644))
	return root
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestDiscoverClusterDNS(t *testing.T) {
	const kubeletCfg = "apiVersion: kubelet.config.k8s.io/v1beta1\nkind: KubeletConfiguration\nclusterDNS:\n- 172.17.0.10\nclusterDomain: cluster.local\n"

	tests := []struct {
		name    string
		setup   func(t *testing.T) clusterDNSSources
		want    string
		wantSrc string
	}{
		{
			name: "config flag path wins",
			setup: func(t *testing.T) clusterDNSSources {
				dir := t.TempDir()
				cfg := writeFile(t, dir, "custom.yaml", kubeletCfg)
				return clusterDNSSources{
					procRoot:       fakeProc(t, "kubelet", "/usr/bin/kubelet", "--config="+cfg),
					defaultConfigs: []string{filepath.Join(dir, "missing.yaml")},
				}
			},
			want:    "172.17.0.10",
			wantSrc: "custom.yaml",
		},
		{
			name: "config flag with space-separated value",
			setup: func(t *testing.T) clusterDNSSources {
				dir := t.TempDir()
				cfg := writeFile(t, dir, "custom.yaml", kubeletCfg)
				return clusterDNSSources{procRoot: fakeProc(t, "kubelet", "kubelet", "--config", cfg)}
			},
			want: "172.17.0.10",
		},
		{
			name: "falls back to default config path when no kubelet process",
			setup: func(t *testing.T) clusterDNSSources {
				dir := t.TempDir()
				cfg := writeFile(t, dir, "config.yaml", kubeletCfg)
				return clusterDNSSources{procRoot: t.TempDir(), defaultConfigs: []string{cfg}}
			},
			want: "172.17.0.10",
		},
		{
			name: "--cluster-dns flag when config has no clusterDNS",
			setup: func(t *testing.T) clusterDNSSources {
				dir := t.TempDir()
				cfg := writeFile(t, dir, "config.yaml", "kind: KubeletConfiguration\n")
				return clusterDNSSources{
					procRoot:       fakeProc(t, "kubelet", "kubelet", "--config="+cfg, "--cluster-dns=10.96.0.10,10.96.0.11"),
					defaultConfigs: []string{cfg},
				}
			},
			want:    "10.96.0.10",
			wantSrc: "--cluster-dns",
		},
		{
			name: "non-IP clusterDNS entry is skipped",
			setup: func(t *testing.T) clusterDNSSources {
				dir := t.TempDir()
				cfg := writeFile(t, dir, "config.yaml", "clusterDNS:\n- not-an-ip\n- 10.0.0.10\n")
				return clusterDNSSources{procRoot: t.TempDir(), defaultConfigs: []string{cfg}}
			},
			want: "10.0.0.10",
		},
		{
			name: "nothing found",
			setup: func(t *testing.T) clusterDNSSources {
				return clusterDNSSources{procRoot: t.TempDir(), defaultConfigs: []string{filepath.Join(t.TempDir(), "nope.yaml")}}
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, src := discoverClusterDNS(tt.setup(t))
			assert.Equal(t, tt.want, got)
			if tt.wantSrc != "" {
				assert.Contains(t, src, tt.wantSrc)
			}
		})
	}
}

func TestIsClusterLocalHost(t *testing.T) {
	tests := []struct {
		host   string
		domain string
		want   bool
	}{
		{"sichek-collector.monitoring.svc.cluster.local", "cluster.local", true},
		{"sichek-collector.monitoring.svc.cluster.local.", "cluster.local", true},
		{"SICHEK-COLLECTOR.MONITORING.SVC.CLUSTER.LOCAL", "cluster.local", true},
		{"a.svc.k8s.internal", "k8s.internal", true},
		{"sichek-collector.monitoring.svc", "cluster.local", false},
		{"10.155.30.38", "cluster.local", false},
		{"example.com", "cluster.local", false},
		{"cluster.local", "cluster.local", false},
		{"evilcluster.local", "cluster.local", false},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			assert.Equal(t, tt.want, isClusterLocalHost(tt.host, tt.domain))
		})
	}
}

// TestClusterDialer_RoutesOnlyClusterLocal verifies that the dialer uses the
// cluster resolver only for cluster-local names, and the default path for
// everything else (IPs included).
func TestClusterDialer_RoutesOnlyClusterLocal(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())

	var looked []string
	d := &clusterDialer{
		domain: "cluster.local",
		lookup: func(ctx context.Context, host string) ([]string, error) {
			looked = append(looked, host)
			return []string{"127.0.0.1"}, nil
		},
		dialer: &net.Dialer{},
	}

	c, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort("svc.ns.svc.cluster.local", port))
	require.NoError(t, err)
	c.Close()
	assert.Equal(t, []string{"svc.ns.svc.cluster.local"}, looked)

	c, err = d.DialContext(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", port))
	require.NoError(t, err)
	c.Close()
	assert.Len(t, looked, 1, "IP address must not go through cluster resolver")
}
