# Simple LB
Operator that allows you to use [Kubernetes LoadBalancer Service](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer) type on non-public cloud, by exposing ports on nodes with `simplelb.amurant.io/enablelb=true` label.


## Howto use?
### 1. Deploy operator
```bash
kubectl apply -f https://github.com/amurant/simplelb/releases/latest/download/simplelb.yaml
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
## Local development
```bash
git clone https://github.com/amurant/simplelb.git
cd simplelb

# create a kind cluster
./devel/create-cluster.sh

# build the docker images and deploy to kind
./devel/install-operator.sh
```

## Based on
 - https://github.com/kontena/akrobateo
 - https://github.com/rancher/k3s/blob/master/pkg/servicelb/controller.go


[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/simplelb)](https://artifacthub.io/packages/search?repo=simplelb)
![GH Actions](https://github.com/amurant/simplelb/actions/workflows/docker-publish.yml/badge.svg)