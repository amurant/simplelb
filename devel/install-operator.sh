#!/usr/bin/env bash

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")
source "${SCRIPT_ROOT}/lib.sh"

cd "${SCRIPT_ROOT}/../"

check_tool docker
check_tool kind
check_tool kubectl
check_tool helm

local_tag="local"

docker build -t "ghcr.io/amurant/simplelb:${local_tag}" .
docker build -t "ghcr.io/amurant/simplelb-portforward:${local_tag}" ./build-portforward/

kind load docker-image --name "${KIND_CLUSTER_NAME}" "ghcr.io/amurant/simplelb:${local_tag}"
kind load docker-image --name "${KIND_CLUSTER_NAME}" "ghcr.io/amurant/simplelb-portforward:${local_tag}"

helm uninstall simplelb
helm install simplelb "${SCRIPT_ROOT}/../deploy/chart/" \
    --set imageSimplelb="ghcr.io/amurant/simplelb:${local_tag}" \
    --set imageSimplelbPortforward="ghcr.io/amurant/simplelb-portforward:${local_tag}"

node_name=$(kubectl get nodes -o=jsonpath='{.items[0].metadata.name}')
kubectl label node "${node_name}" simplelb.amurant.io/enablelb=true

kubectl create deployment hello-node --image=k8s.gcr.io/echoserver:1.4
kubectl expose deployment hello-node --type=LoadBalancer --port=8080
