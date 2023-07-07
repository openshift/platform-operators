#!/bin/bash

set -o nounset
set -o pipefail
set -e

export ARTIFACT_DIR=${ARTIFACT_DIR:-/tmp}
export PO_NAMESPACE="openshift-platform-operators"

KUBECTL=${KUBECTL:=kubectl}

function cleanup() {
    set +e +o pipefail
    exit_status=$?

    echo "Performing cleanup"

    echo "Stopping background jobs"
    # kill any background jobs
    pids=$(jobs -pr)
    local pids
    [ -n "$pids" ] && kill -9 "$pids"
    # Wait for any jobs
    wait 2>/dev/null

    echo "Exiting $0"
    exit "$exit_status"
}

trap cleanup SIGINT

function applyFeatureGate() {
  echo "$(date -u --rfc-3339=seconds) - Apply TechPreviewNoUpgrade FeatureGate configuration"

cat <<EOF | ${KUBECTL} apply -f -
---
apiVersion: config.openshift.io/v1
kind: FeatureGate
metadata:
  annotations:
    include.release.openshift.io/self-managed-high-availability: "true"
    include.release.openshift.io/single-node-developer: "true"
    release.openshift.io/create-only: "true"
  name: cluster
spec:
  featureSet: TechPreviewNoUpgrade
EOF
}

function waitForAggregatedPlatformOperatorCORollout() {
  echo "$(date -u --rfc-3339=seconds) - Wait for the platform operator aggregated ClusterOperator to go available..."
  waitFor 10m ${KUBECTL} wait --for=condition=Available=True clusteroperators.config.openshift.io/platform-operators-aggregated
}

function waitForRunningPod() {
  local REGEXP="${1}"
  local MSG="${1}"

  while [ "$(${KUBECTL} get pods -n ${PO_NAMESPACE} -o name | grep -c "${REGEXP}")" == 0 ]; do
    echo "$(date -u --rfc-3339=seconds) - ${MSG}"
    sleep 5
  done
}
export -f waitForRunningPod

function ClusterPlatformOperatorPodsCreated() {
  waitForRunningPod "platform-operators-rukpak-core" "Waiting for rukpak core creation"
  waitForRunningPod "platform-operators-rukpak-webhooks" "Waiting for rukpak webhooks creation"
}
export -f ClusterPlatformOperatorPodsCreated

function waitForClusterPlatformOperatorPodsReadiness() {
  echo "$(date -u --rfc-3339=seconds) - Wait for PO operands to be ready"
  waitFor 10m ${KUBECTL} wait --all -n "${PO_NAMESPACE}" --for=condition=ready pods
}

function waitFor() {
  local TIMEOUT="${1}"
  local CMD="${*:2}"

  ret=0
  timeout "${TIMEOUT}" bash -c "execute ${CMD}" || ret="$?"

  # Command timed out
  if [[ ret -eq 124 ]]; then
    echo "$(date -u --rfc-3339=seconds) - Timed out waiting for result of $CMD"
    exit 1
  fi
}

function execute() {
  local CMD="${*}"

  # API server occasionally becomes unavailable, so we repeat command in case of error
  while true; do
    ret=0
    ${CMD} || ret="$?"

    if [[ ret -eq 0 ]]; then
      return
    fi

    echo "$(date -u --rfc-3339=seconds) - Command returned error $ret, retrying..."
  done
}
export -f execute

applyFeatureGate
waitFor 30m ClusterPlatformOperatorPodsCreated
waitForClusterPlatformOperatorPodsReadiness
waitForAggregatedPlatformOperatorCORollout
