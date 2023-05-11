LOCAL_BIN:=$(CURDIR)/bin
GO_VERSION:=$(shell go version)
GOLANGCI_BIN:=$(LOCAL_BIN)/golangci-lint
GOX_BIN:=$(LOCAL_BIN)/gox
BUILD_ENVPARMS:=CGO_ENABLED=0
PROTOGEN_BIN:=$(LOCAL_BIN)/protogen

ifndef HOSTOS
HOSTOS:=$(shell go env GOHOSTOS)
endif

ifndef HOSTARCH
HOSTARCH:=$(shell go env GOHOSTARCH)
endif

.PHONY: .install-utils
.install-utils:
	go mod tidy
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

.PHONY: install-utils
install-utils: .install-utils

.PHONY: install-lint
install-lint:
ifeq ($(wildcard $(GOLANGCI_BIN)),)
	$(info Downloading golangci-lint latest)
	GOBIN=$(LOCAL_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
GOLANGCI_BIN:=$(LOCAL_BIN)/golangci-lint
endif

.PHONY: .lint
.lint: install-lint
	$(info Running lint...)
	$(GOLANGCI_BIN) cache clean && go clean -modcache -cache -i
	$(GOLANGCI_BIN) run --config=.golangci.pipeline.yml ./...

.PHONY: lint
lint: .lint

.PHONY: .install-gox
.install-gox:
	GOBIN=$(LOCAL_BIN) go install github.com/mitchellh/gox@latest

.PHONY: install-gox
install-gox: .install-gox

.PHONY: .test-build
.test-build:
	$(info Building...)
	$(BUILD_ENVPARMS) $(GOX_BIN) -output="$(BIN_DIR)/{{.Dir}}" -osarch="$(HOSTOS)/$(HOSTARCH)" ./cmd/microservice

.PHONY: test-build
test-build: .test-build

.PHONY: .build
.build:
	GOOS=linux go build -o $(LOCAL_BIN)/microservice ./cmd/microservice

.PHONY: build
build: .build

.PHONY: .test
.test:
	$(info Running tests...)
	go test ./...

# run unit tests
.PHONY: test
test: .test

.PHONY: .generate
.generate:
	protoc -I ./internal/pb -I ./api \
       --go_out ./pkg/microservice --go_opt paths=source_relative \
       --go-grpc_out ./pkg/microservice --go-grpc_opt paths=source_relative \
       --grpc-gateway_out ./pkg/microservice --grpc-gateway_opt paths=source_relative \
       --openapiv2_out ./api --openapiv2_opt logtostderr=true \
       ./api/microservice.proto

.PHONY: generate
generate: .generate

build-image:
	docker build -t microservice .
	docker image tag microservice imigaka/microservice:latest
	docker push imigaka/microservice:latest

run: build-image
	docker compose build --no-cache
	docker compose up -d
#	docker compose logs -f

stop:
	docker compose stop

down:
	docker compose down

logs:
	docker compose logs -f

restart: down run

swarm-run: build-image
	docker swarm init
	env GRAYLOG_PASSWORD_SECRET={$GRAYLOG_PASSWORD_SECRET} GRAYLOG_ROOT_PASSWORD_SHA2={$GRAYLOG_ROOT_PASSWORD_SHA2} docker stack deploy -c docker-compose.yml microservice

swarm-down:
	docker stack rm microservice
	docker swarm leave --force