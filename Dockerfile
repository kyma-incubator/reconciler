FROM eu.gcr.io/kyma-project/external/golang:1.16.3-alpine as builder

ENV BASE_APP_DIR /go/src/github.com/kyma-incubator/reconciler
WORKDIR ${BASE_APP_DIR}

ENV GO111MODULES=on

COPY ./go.mod ${BASE_APP_DIR}/go.mod
COPY ./go.sum ${BASE_APP_DIR}/go.sum

# Run go mod download first to take advantage of Docker caching
RUN apk add build-base
RUN apk add git && go mod download
RUN apk add curl

COPY ./cmd/ ${BASE_APP_DIR}/cmd
COPY ./configs/ ${BASE_APP_DIR}/cmd/configs
COPY ./pkg/ ${BASE_APP_DIR}/pkg/
COPY ./internal/ ${BASE_APP_DIR}/internal/
COPY ./lib/ ${BASE_APP_DIR}/lib

RUN apk add -U --no-cache ca-certificates && update-ca-certificates

RUN go build -v -o main ./cmd/
RUN mkdir /app && mv ./main /app/main &&
# Add kubectl
RUN mv ./lib/kubectl /app/kubectl && chmod +x /app/kubectl

FROM eu.gcr.io/kyma-project/external/alpine:3.13.5

WORKDIR /app

COPY --from=builder /app /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/app/main","service","start"]
