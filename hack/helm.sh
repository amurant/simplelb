#!/usr/bin/env bash

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")
source "${SCRIPT_ROOT}/../devel/lib.sh"

check_tool helm

helm $@
