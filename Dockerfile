FROM registry.ci.openshift.org/ocp/builder:rhel-8-golang-1.20-openshift-4.14 AS builder

WORKDIR /build
COPY vendor vendor
COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd cmd
COPY api api
COPY internal internal
COPY Makefile Makefile
RUN make build

FROM registry.ci.openshift.org/ocp/4.14:base

COPY manifests /manifests

# ncdc: temporarily commenting out this label so this image is not included in the release payload. We are moving the
# rukpak manifests to openshift/cluster-olm-operator and have to do it through a series of PRs. The easiest way to do
# the move is to stop including platform-operators in the payload, then add it back after the move is done.
# LABEL io.openshift.release.operator=true

COPY --from=builder /build/bin/manager /
USER 1001

LABEL io.k8s.display-name="OpenShift Platform Operator Manager" \
      io.k8s.description="This is a component of OpenShift Container Platform and manages the lifecycle of platform operators." \
      io.openshift.tags="openshift" \
      summary="This is a component of OpenShift Container Platform and manages the lifecycle of platform operators." \
      maintainer="Odin Team <aos-odin@redhat.com>"
