# Istioctl source images
FROM istio/istioctl:1.10.2 AS istio-1_10_2

# Build image
FROM golang:1.16.4-alpine3.12 AS build

ENV SRC_DIR=/go/src/github.com/kyma-incubator/reconciler
ADD . $SRC_DIR

RUN mkdir /user && \
    echo 'appuser:x:2000:2000:appuser:/:' > /user/passwd && \
    echo 'appuser:x:2000:' > /user/group

WORKDIR $SRC_DIR

COPY configs /configs
RUN CGO_ENABLED=0 go build -o /bin/reconciler ./cmd/main.go

# Get latest CA certs
FROM alpine:latest as certs
RUN apk --update add ca-certificates

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
COPY --from=istio-1_10_2 /usr/local/bin/istioctl /bin/istioctl-1.10.2

USER appuser:appuser

CMD ["/bin/reconciler"]
