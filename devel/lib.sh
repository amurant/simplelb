#!/usr/bin/env bash

LIB_SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")
TOOLS_PATH="${LIB_SCRIPT_ROOT}/bin"
PATH="$PATH:$TOOLS_PATH"

fn_exists() { declare -F "$1" > /dev/null; }

check_tool() {
  tool="$1"
  if command -v "$tool" &>/dev/null; then
    return 0 # tool was found, just return
  fi

  if ! fn_exists "install_tool_${tool}"; then
    echo "ERROR: tool ${tool} not found"
    exit 1
  fi

  mkdir -p "${TOOLS_PATH}"

  # we have to install the tool
  echo "INSTALLING: ${tool}"
  
  $("install_tool_${tool}")
}

install_tool_kind() {
  curl -Lo "${TOOLS_PATH}/kind" "https://kind.sigs.k8s.io/dl/v0.11.1/kind-linux-amd64"
  chmod +x "${TOOLS_PATH}/kind"
}

install_tool_kubectl() {
  curl -Lo "${TOOLS_PATH}/kubectl" "https://dl.k8s.io/release/v1.21.0/bin/linux/amd64/kubectl"
  chmod +x "${TOOLS_PATH}/kubectl"
}

install_tool_helm() {
  pushd "${TOOLS_PATH}"

  curl -Lo helm.tar.gz "https://get.helm.sh/helm-v3.6.2-linux-amd64.tar.gz"

  tar -zxvf helm.tar.gz

  mv ./linux-amd64/helm "./helm"

  rm -rf ./linux-amd64/
  rm -rf helm.tar.gz
  popd
}

# Config variables
KIND_CLUSTER_NAME="simplelb"
