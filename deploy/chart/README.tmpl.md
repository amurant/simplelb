# Simple LB
Operator that allows you to use [Kubernetes LoadBalancer Service](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) type on non-public cloud, by exposing ports on nodes with `simplelb.amurant.io/enablelb=true` label.

> Checkout on Github:
>
> <https://github.com/amurant/simplelb>

## Howto use?
### 1. Deploy operator
```bash
export HELM_EXPERIMENTAL_OCI=1

helm chart pull ghcr.io/amurant/simplelb-helm:${VERSION}
helm chart export ghcr.io/amurant/simplelb-helm:${VERSION}
helm install simplelb ./simplelb-helm
```

### 2. Label nodes
```bash
kubectl label node <node name> simplelb.amurant.io/enablelb=true
kubectl get nodes --show-labels
# remove a label:
kubectl label node <node name> simplelb.amurant.io/enablelb-
```
### 3. Deploy example app (optional)
```bash
kubectl create deployment hello-node --image=k8s.gcr.io/echoserver:1.4

kubectl expose deployment hello-node --type=LoadBalancer --port=8080
```

## Based on
 - https://github.com/kontena/akrobateo
 - https://github.com/rancher/k3s/blob/master/pkg/servicelb/controller.go


[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/simplelb)](https://artifacthub.io/packages/search?repo=simplelb)
![GH Actions](https://github.com/amurant/simplelb/actions/workflows/docker-publish.yml/badge.svg)