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
lint:
	./scripts/lint.sh

.PHONY: build
build: build-linux build-darwin build-linux-arm

.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./bin/configmgt-linux $(FLAGS) ./cmd

.PHONY: build-linux-arm
build-linux-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o ./bin/configmgt-arm $(FLAGS) ./cmd

.PHONY: build-darwin
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o ./bin/configmgt-darwin $(FLAGS) ./cmd

.PHONY: test
test:
	go test -race -coverprofile=cover.out ./...
	@echo "Total test coverage: $$(go tool cover -func=cover.out | grep total | awk '{print $$3}')"
	@rm cover.out

.PHONY: clean
clean:
	rm -rf bin

.PHONY: all
all: resolve build test lint 