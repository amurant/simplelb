#!/usr/bin/env bash

git tag -l | xargs git tag -d
git fetch --all --tags
git tag -a $1 -m "$1" && \
git push origin --tags
