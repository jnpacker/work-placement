# Build the manager binary
FROM registry.ci.openshift.org/open-cluster-management/builder:go1.17-linux as builder

WORKDIR /go/src/github.com/open-cluster-management-io/work-placement
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY pkg/main.go pkg/main.go
COPY api/ api/
COPY pkg/controllers/ pkg/controllers/
COPY config/ config/
COPY Makefile Makefile
COPY hack hack

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

# Add the binaries
COPY --from=builder /go/src/github.com/open-cluster-management-io/work-placement/bin/manager .
ENV USER_UID=1001

ENTRYPOINT ["/manager"]
