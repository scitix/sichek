# `k8s/deploy.yaml` README

This document explains how to render, apply, and operate `k8s/deploy.yaml`.

## What this manifest contains

`k8s/deploy.yaml` defines:

- A `DaemonSet` named `sichek-gpu` in namespace `monitoring`
- A `ServiceAccount` (`sa-sichek`)
- A `ClusterRole` and `ClusterRoleBinding`
- A `PodMonitor` (`sichek-exporter`)

The DaemonSet has:

- `initContainer` (`sichek-init`) that installs/verifies `sichek` on the host
- Main container (`sichek`) that keeps host-side `sichek` service alive
- Exporter container (`sichek-exporter`) that exposes metrics over HTTP

Important runtime characteristics:

- `hostPID: true`
- `hostNetwork: true`
- `privileged: true`
- Uses `nsenter` to execute commands in host namespaces
- Uses hostPath `/var/sichek` (mounted as `/host/var/sichek` in containers)

## Template variables you must render

`k8s/deploy.yaml` is templated. Replace these placeholders before apply:

- `{{ .registry }}`: image registry, for example `registry.example.com`
- `{{ .version }}`: sichek version tag, for example `v0.7.6`
- `{{ .sichek_spec_url }}`: spec fallback URL (or empty string)
- `{{ .metrics_port }}`: exporter metrics port, for example `19092`

## Prerequisites

1. Namespace exists:

```bash
kubectl create namespace monitoring
```

2. ConfigMaps exist in `monitoring`:

- `sichek-default-spec` with key `default_spec.yaml`
- `sichek-default-user-config` with key `default_user_config.yaml`

Example:

```bash
kubectl create configmap sichek-default-spec \
  --from-file=default_spec.yaml=/path/to/default_spec.yaml \
  -n monitoring

kubectl create configmap sichek-default-user-config \
  --from-file=default_user_config.yaml=/path/to/default_user_config.yaml \
  -n monitoring
```

3. Cluster has permissions and binaries expected by the script on target nodes:

- `systemd`/`systemctl`
- package manager (`rpm` or `dpkg`)
- host can run `sichek` after install

## Render and apply

From repository root:

```bash
export REGISTRY="registry-ap-southeast.scitix.ai"
export SICHEK_VERSION="v0.7.6"
export SICHEK_SPEC_URL='""'
export METRICS_PORT="19092"

sed -e "s|{{ \\.registry }}|${REGISTRY}|g" \
    -e "s|{{ \\.version }}|${SICHEK_VERSION}|g" \
    -e "s|{{ \\.sichek_spec_url }}|${SICHEK_SPEC_URL}|g" \
    -e "s|{{ \\.metrics_port }}|${METRICS_PORT}|g" \
    k8s/deploy.yaml > k8s/deploy.rendered.yaml

kubectl apply -f k8s/deploy.rendered.yaml
```

## Verify deployment

```bash
kubectl get daemonset -n monitoring sichek-gpu
kubectl get pods -n monitoring -l app=sichek -o wide

# init/install logs
kubectl logs -n monitoring <pod-name> -c sichek-init

# keepalive service logs
kubectl logs -n monitoring <pod-name> -c sichek -f

# exporter logs
kubectl logs -n monitoring <pod-name> -c sichek-exporter -f
```

## Update flow

### Update config only

1. Update ConfigMaps
2. Restart DaemonSet:

```bash
kubectl rollout restart daemonset/sichek-gpu -n monitoring
```

### Upgrade version

1. Change rendered values (`REGISTRY` / `SICHEK_VERSION`)
2. Re-render and re-apply
3. Watch rollout:

```bash
kubectl rollout status daemonset/sichek-gpu -n monitoring
```

## Uninstall

```bash
kubectl delete -f k8s/deploy.rendered.yaml
```

Note: this removes Kubernetes resources, but host files under `/var/sichek` and host-installed packages may remain.

## Troubleshooting quick notes

- `Init` fails: check `sichek-init` logs first.
- `sichek` keeps restarting: verify host `systemctl is-active sichek`.
- Exporter exits with socket timeout: check `/var/sichek/run/current/metrics.sock` on host.
- Pod cannot schedule: inspect taints/resources and DaemonSet events.
