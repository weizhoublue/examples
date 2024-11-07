#!/bin/bash
# Copyright 2024 Authors of elf-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail
set -x

K8S_VERSION=$( kubeadm version | egrep -o "GitVersion[^,]+" | awk -F'"' '{print $2}' | tr -d '\n' )
kubeadm config images list --kubernetes-version ${K8S_VERSION}

GCR_URL=registry.k8s.io
ALIYUN_URL=registry.cn-hangzhou.aliyuncs.com/google_containers
ORIGIN_IMAGES=$(kubeadm config images list  --kubernetes-version ${K8S_VERSION} 2>/dev/null)
EXISTED_IMAGES=$(podman images --format "{{.Repository}}:{{.Tag}}" | sed -E 's/[[:space:]]+/:/')

TIMEOUT_SEC=120

echo "---------- origin images: ----------"
echo  "${ORIGIN_IMAGES} "
echo ""

IMAGES=$(grep -v 'coredns' <<< "${ORIGIN_IMAGES}")
for ITEM in ${IMAGES}; do
  if grep -q "${ITEM}" <<< "${EXISTED_IMAGES}"; then
    echo "---------- image existed: ${ITEM} ----------"
    continue
  fi

  echo "---------- pull image: ${ITEM} ----------"
  DOWNLOAD_IMAGE=$(echo "$ITEM" | sed "s?${GCR_URL}?${ALIYUN_URL}?g")
  timeout ${TIMEOUT_SEC} podman pull $DOWNLOAD_IMAGE || timeout ${TIMEOUT_SEC} podman pull $DOWNLOAD_IMAGE || timeout ${TIMEOUT_SEC} podman pull $DOWNLOAD_IMAGE
  podman tag $DOWNLOAD_IMAGE $ITEM
done

COREDNS_IMAGES=$(grep 'coredns' <<< "${ORIGIN_IMAGES}")
if ! grep -q "${COREDNS_IMAGES}" <<< "${EXISTED_IMAGES}"; then
  echo "---------- pull image: ${COREDNS_IMAGES} ----------"
  # coredns image is special
  DOWNLOAD_DNS_IMAGE=$(echo "${COREDNS_IMAGES}" | sed "s?${GCR_URL}?${ALIYUN_URL}?g" | sed 's?coredns/coredns?coredns?')
  timeout ${TIMEOUT_SEC} podman pull $DOWNLOAD_DNS_IMAGE || timeout ${TIMEOUT_SEC} podman pull $DOWNLOAD_DNS_IMAGE || timeout ${TIMEOUT_SEC} podman pull $DOWNLOAD_DNS_IMAGE
  podman tag $DOWNLOAD_DNS_IMAGE ${COREDNS_IMAGES}
else
  echo "---------- image existed: ${COREDNS_IMAGES} ----------"
fi

exit 0
