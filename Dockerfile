FROM registry.ci.openshift.org/ocp/builder:rhel-8-golang-1.18-openshift-4.12 AS builder

WORKDIR /build
COPY vendor vendor
COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd cmd
COPY api api
COPY controllers controllers
COPY internal internal
COPY Makefile Makefile
RUN make build

FROM registry.ci.openshift.org/ocp/4.12:base

COPY manifests /manifests
# LABEL io.openshift.release.operator=true

COPY --from=builder /build/bin/manager /
USER 1001

LABEL io.k8s.display-name="OpenShift Platform Operator Manager" \
      io.k8s.description="This is a component of OpenShift Container Platform and manages the lifecycle of platform operators." \
      io.openshift.tags="openshift" \
      summary="This is a component of OpenShift Container Platform and manages the lifecycle of platform operators." \
      maintainer="Odin Team <aos-odin@redhat.com>"
