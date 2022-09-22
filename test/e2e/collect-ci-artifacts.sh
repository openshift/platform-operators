#! /bin/bash

set -o pipefail
set -o nounset
set -o errexit

: "${KUBECONFIG:?}"
: "${ARTIFACT_DIR:?}"
: "${KUBECTL:=kubectl}"

echo "Using the ${KUBECTL} kubectl binary"
echo "Using the ${ARTIFACT_DIR} output directory"
mkdir -p "${ARTIFACT_DIR}"

commands=()
commands+=("get co platform-operators-aggregated -o yaml")
commands+=("get platformoperators -o yaml")
commands+=("get bundledeployments -o yaml")
commands+=("get bundles -o yaml")

echo "Storing the test artifact output in the ${ARTIFACT_DIR} directory"
for command in "${commands[@]}"; do
    echo "Collecting ${command} output..."
    COMMAND_OUTPUT_FILE=${ARTIFACT_DIR}/${command// /_}

    ${KUBECTL} ${command} >> "${COMMAND_OUTPUT_FILE}"
done
