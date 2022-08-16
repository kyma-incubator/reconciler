# Istioctl source images
FROM eu.gcr.io/kyma-project/external/istio/istioctl:1.12.3 AS istio-1_12_3
FROM eu.gcr.io/kyma-project/external/istio/istioctl:1.13.2 AS istio-1_13_2
FROM eu.gcr.io/kyma-project/external/istio/istioctl:1.14.1 AS istio-1_14_1

# Build image
FROM golang:1.18.1-alpine3.15 AS build

ENV SRC_DIR=/go/src/github.com/kyma-incubator/reconciler
COPY . $SRC_DIR

RUN mkdir /user && \
    echo 'appuser:x:2000:2000:appuser:/:' > /user/passwd && \
    echo 'appuser:x:2000:' > /user/group

WORKDIR $SRC_DIR

COPY configs /configs
RUN CGO_ENABLED=0 go build -o /bin/reconciler -ldflags '-s -w' ./cmd/reconciler/main.go

# Get latest CA certs
# hadolint ignore=DL3007
FROM alpine:latest as certs
RUN apk add --no-cache ca-certificates

# Final image
FROM scratch
LABEL source=git@github.com:kyma-incubator/reconciler.git

# Add SSL certificates
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Add system users
COPY --from=build /user/group /user/passwd /etc/

# Add reconciler
COPY --from=build /bin/reconciler /bin/reconciler
COPY --from=build /configs/ /configs/

# Add istioctl tools
COPY --from=istio-1_12_3 /usr/local/bin/istioctl /bin/istioctl-1.12.3
COPY --from=istio-1_13_2 /usr/local/bin/istioctl /bin/istioctl-1.13.2
COPY --from=istio-1_14_1 /usr/local/bin/istioctl /bin/istioctl-1.14.1
# For multiple istioctl binaries, provide their paths separated with a semicolon (;) like in the Linux PATH variable.
ENV ISTIOCTL_PATH=/bin/istioctl-1.12.3;/bin/istioctl-1.13.2;/bin/istioctl-1.14.1

USER appuser:appuser

CMD ["/bin/reconciler"]
