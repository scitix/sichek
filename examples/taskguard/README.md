# TaskGuard

## build

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o bin/taskguard
```

## run

```bash
bin/taskguard
```

## config
The config in etc/config.yaml
- EnableTaskGuardLabel: Set which pytorchjob tasks to enable anomaly detection and restart functionality based on labels. If not set, it will apply to all pytorchjobs.
- KubeConfig.ConfigFile: Set your own kube config file or remove it if it runs in k8s cluster
- FaultToleranceConfig.EnableTaskGuardLabel: Check the task with this label, remove it if you want to check all the task