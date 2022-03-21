#!/bin/bash
#
# This builds a version of the pachd and worker images based on Red Hat's UBI,
# rather than on 'scratch'.

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
# shellcheck disable=SC1090
source "${SCRIPT_DIR}/../govars.sh"

set -x

if [[ -z "${REDHAT_MARKETPLACE_PACHD_OSPID}" ]]; then
  echo "Must set REDHAT_MARKETPLACE_PACHD_OSPID" >/dev/stderr
  exit 1
fi

# Generate Dockerfiles based on RedHat's UBI instead of scratch.
#
# This is effectively required for our RedHat Marketplace offering (as
# implemented by our OpenShift operator in
# github.com/pachyderm/openshift-operator. Users are given an interface that
# allows them to run a shell in their pachd/worker containers, and RedHat's
# Universal Base Image provides a shell they can run while conforming to the
# the RedHat Marketplace image approval rules)
for img in pachd worker; do
  sed \
    's#FROM scratch#FROM registry.access.redhat.com/ubi8/ubi-minimal#g' \
    "Dockerfile.${img}" \
    >"Dockerfile.redhat.${img}"
done

# Build images from the modified Dockerfiles
make docker-build

# Determine the appropriate image tag version (using the same logic as
# etc/build/make_release.sh)
#
# TODO(msteffen) Use '?=' syntax in makefile to eliminate the need for this check
if [[ "${VERSION_ADDITIONAL+true}" = "true" ]]; then
  make VERSION_ADDITIONAL="${VERSION_ADDITIONAL}" install
else
  make install
fi
version="$("${PACHCTL}" version --client-only)"

# Push the image to our Red Hat Technology Portal project
for img in pachd worker; do
  ospid=$(eval "echo \${REDHAT_MARKETPLACE_${img@U}_OSPID}")
  password=$(eval "echo \${REDHAT_MARKETPLACE_${img@U}_PASSWORD}")
  docker login -u unused scan.connect.redhat.com --password-stdin <<<"${password}"
  docker tag "pachyderm/${img}:local" "scan.connect.redhat.com/${ospid}/${img}:${version}"
  docker push "scan.connect.redhat.com/${ospid}/${img}:${version}"
done
