#! /bin/bash

set -o pipefail
set -o nounset
set -o errexit

: "${KUBECONFIG:?}"
: "${ARTIFACT_DIR:?}"
: "${KUBECTL:=oc}"

function ensure_kubectl() {
    # Check whether we're running in a CI environment as the "oc" binary
    # location is located at /cli/oc but that /cli directly doesn't exist
    # in the $PATH variable.
    if [[ "$OPENSHIFT_CI" == "true" ]]; then
        echo "Detected the e2e suite is being run in CI environment. Adding the /cli to \$PATH"
        export PATH=$PATH:/cli
    fi

    if ! which ${KUBECTL} &> /dev/null; then
        echo "cannot find the configured ${KUBECTL} binary in \$PATH"
        exit 1
    fi
}

function collect_artifacts() {
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
}

function main() {
    echo "Using the ${KUBECTL} kubectl binary"
    echo "Using the ${ARTIFACT_DIR} output directory"
    mkdir -p "${ARTIFACT_DIR}"

    ensure_kubectl
    collect_artifacts
}

main
