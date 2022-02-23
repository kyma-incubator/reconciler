# Istioctl source images
FROM eu.gcr.io/kyma-project/external/istio/istioctl:1.11.4 AS istio-1_11_4

# Build image
FROM golang:1.17.7-alpine3.15 AS build

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
COPY --from=istio-1_11_4 /usr/local/bin/istioctl /bin/istioctl-1.11.4
# For multiple istioctl binaries, provide their paths separated with a colon (:) like in the Linux PATH variable.
ENV ISTIOCTL_PATH=/bin/istioctl-1.11.4

USER appuser:appuser

CMD ["/bin/reconciler"]
