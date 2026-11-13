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
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// The sichek daemon runs on the host (systemd), so it uses the host's
// /etc/resolv.conf, which knows nothing about Kubernetes Service names. To let
// the reporter target `<svc>.<ns>.svc.cluster.local`, we discover the cluster
// DNS server from the local kubelet and use it only for cluster-local names.

const defaultClusterDomain = "cluster.local"

// clusterDNSSources lists where to look for kubelet's clusterDNS. Fields are
// injectable for tests.
type clusterDNSSources struct {
	procRoot       string   // usually /proc
	defaultConfigs []string // kubelet config files tried after the --config flag
}

func defaultClusterDNSSources() clusterDNSSources {
	return clusterDNSSources{
		procRoot:       "/proc",
		defaultConfigs: []string{"/var/lib/kubelet/config.yaml"},
	}
}

type kubeletConfig struct {
	ClusterDNS    []string `yaml:"clusterDNS"`
	ClusterDomain string   `yaml:"clusterDomain"`
}

// discoverClusterDNS returns the first cluster DNS IP and a description of
// where it came from. Order: kubelet --config file, default kubelet config
// files, kubelet --cluster-dns flag. Returns "" when nothing is found.
func discoverClusterDNS(src clusterDNSSources) (string, string) {
	ip, _, from := discoverClusterDNSWithDomain(src)
	return ip, from
}

func discoverClusterDNSWithDomain(src clusterDNSSources) (ip, domain, from string) {
	args := kubeletArgs(src.procRoot)

	configs := make([]string, 0, len(src.defaultConfigs)+1)
	if p := flagValue(args, "--config"); p != "" {
		configs = append(configs, p)
	}
	configs = append(configs, src.defaultConfigs...)

	seen := map[string]bool{}
	for _, p := range configs {
		if seen[p] {
			continue
		}
		seen[p] = true
		kc, err := readKubeletConfig(p)
		if err != nil {
			continue
		}
		for _, s := range kc.ClusterDNS {
			if net.ParseIP(strings.TrimSpace(s)) != nil {
				return strings.TrimSpace(s), kc.ClusterDomain, p
			}
		}
	}

	for _, s := range strings.Split(flagValue(args, "--cluster-dns"), ",") {
		if net.ParseIP(strings.TrimSpace(s)) != nil {
			return strings.TrimSpace(s), flagValue(args, "--cluster-domain"), "kubelet --cluster-dns"
		}
	}
	return "", "", ""
}

func readKubeletConfig(path string) (kubeletConfig, error) {
	var kc kubeletConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return kc, err
	}
	err = yaml.Unmarshal(data, &kc)
	return kc, err
}

// kubeletArgs returns the argv of the first process whose comm is "kubelet",
// or nil.
func kubeletArgs(procRoot string) []string {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		name := e.Name()
		if name == "" || name[0] < '0' || name[0] > '9' {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(procRoot, name, "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "kubelet" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(procRoot, name, "cmdline"))
		if err != nil {
			continue
		}
		var args []string
		for _, a := range bytes.Split(bytes.TrimRight(raw, "\x00"), []byte{0}) {
			args = append(args, string(a))
		}
		return args
	}
	return nil
}

// flagValue supports both "--flag=value" and "--flag value".
func flagValue(args []string, flag string) string {
	for i, a := range args {
		if v, ok := strings.CutPrefix(a, flag+"="); ok {
			return v
		}
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// isClusterLocalHost reports whether host is a name under the cluster domain.
func isClusterLocalHost(host, domain string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	return strings.HasSuffix(h, "."+strings.ToLower(domain))
}

// clusterDialer resolves cluster-local names via the cluster DNS and leaves
// every other address to the default dialer (host resolver).
type clusterDialer struct {
	domain string
	lookup func(ctx context.Context, host string) ([]string, error)
	dialer *net.Dialer
}

func (d *clusterDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || !isClusterLocalHost(host, d.domain) {
		return d.dialer.DialContext(ctx, network, addr)
	}
	ips, err := d.lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("cluster dns lookup %s: %w", host, err)
	}
	var lastErr error
	for _, ip := range ips {
		c, err := d.dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
		if err == nil {
			return c, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no addresses for %s", host)
	}
	return nil, lastErr
}

// newReporterTransport returns an http.Transport whose dialer resolves
// cluster-local names through the kubelet-configured cluster DNS. When no
// cluster DNS is discovered it logs a warning and uses the host resolver.
func newReporterTransport(endpointHost string, src clusterDNSSources) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	base := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	dnsIP, domain, from := discoverClusterDNSWithDomain(src)
	if domain == "" {
		domain = defaultClusterDomain
	}
	if !isClusterLocalHost(endpointHost, domain) {
		return tr
	}
	if dnsIP == "" {
		logrus.WithField("service", "reporter").Warnf(
			"endpoint host %s is cluster-local but no kubelet clusterDNS found; falling back to host resolver", endpointHost)
		return tr
	}
	logrus.WithField("service", "reporter").Infof(
		"resolving *.%s via cluster DNS %s (from %s)", domain, dnsIP, from)

	server := net.JoinHostPort(dnsIP, "53")
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return base.DialContext(ctx, network, server)
		},
	}
	tr.DialContext = (&clusterDialer{
		domain: domain,
		lookup: resolver.LookupHost,
		dialer: base,
	}).DialContext
	return tr
}
