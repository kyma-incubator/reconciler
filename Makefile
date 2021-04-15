ifndef VERSION
	VERSION = ${shell git describe --tags --always}
endif

ifeq ($(VERSION),stable)
	VERSION = stable-${shell git rev-parse --short HEAD}
endif

.DEFAULT_GOAL=all

.PHONY: resolve
resolve:
	go mod tidy

.PHONY: lint
validate:
	./scripts/lint.sh

.PHONY: build
build: build-linux build-darwin build-linux-arm

.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/cfgmgt-linux $(FLAGS) ./cmd

.PHONY: build-linux-arm
build-linux-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ./bin/cfgmgt-arm $(FLAGS) ./cmd

.PHONY: build-darwin
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ./bin/cfgmgt-darwin $(FLAGS) ./cmd

.PHONY: all
all: resolve build lint 