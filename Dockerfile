# Istioctl source images
FROM istio/istioctl:1.11.2 AS istio-1_11_2

# Build image
FROM golang:1.17.0-alpine3.13 AS build

ENV SRC_DIR=/go/src/github.com/kyma-incubator/reconciler
ADD . $SRC_DIR

RUN mkdir /user && \
    echo 'appuser:x:2000:2000:appuser:/:' > /user/passwd && \
    echo 'appuser:x:2000:' > /user/group

WORKDIR $SRC_DIR

COPY configs /configs
RUN CGO_ENABLED=0 go build -o /bin/reconciler -ldflags '-s -w' ./cmd/main.go

RUN apk update && apk upgrade && \
    apk --no-cache add curl
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.15.0-beta.1/migrate.linux-386.tar.gz -o migrate.tar.gz &&\
    tar xvzf migrate.tar.gz migrate -C /bin/ &&\
    ls -la /bin &&\
    chmod +x /bin/migrate

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
COPY --from=build /bin/migrate /bin/migrate
COPY --from=build /configs/ /configs/

# Add istioctl tools
COPY --from=istio-1_11_2 /usr/local/bin/istioctl /bin/istioctl-1.11.2
ENV ISTIOCTL_PATH=/bin/istioctl-1.11.2

USER appuser:appuser

CMD ["/bin/reconciler"]
