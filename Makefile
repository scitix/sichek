.PHONY: docker
PROJECT_NAME := sichek
GO := go
INSTALL_DIR := /usr/local/bin

# Version variables (can be overridden)
GIT_COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y%m%dT%H%M%SZ')
VERSION := dev-${BUILD_TIME}-${GIT_COMMIT}
TASKGUARD_VERISON := v0.1.0
SICL_PKG_VERSION := v0.2.1
SICL_PKG_NAME := sicl-nccl2.29.2-1-cuda12.9-ompi4.1.8-ubuntu22.04-20260128.run

all:
	mkdir -p build/bin/
	go mod tidy
	go mod vendor
	GOOS=linux GOARCH=amd64 $(GO) build -gcflags "all=-N -l" -o build/bin/$(PROJECT_NAME) cmd/main.go

goreleaser:
	BUILD_TIME=${BUILD_TIME} \
	SICL_PKG_VERSION=${SICL_PKG_VERSION} \
	SICL_PKG_NAME=${SICL_PKG_NAME} \
	goreleaser release --snapshot --clean
# 	ossctl cp dist/sichek_0.0.0~${VERSION}_linux_amd64.deb shanghai-2/hisys:hisys-sichek-sh/dev/sichek_0.0.0~${VERSION}_linux_amd64.deb
	ossctl cp dist/sichek_0.0.0~${VERSION}_linux_amd64.deb scitix_oss/hisys-sichek/dev/0.0.0~${VERSION}/sichek_0.0.0~${VERSION}_linux_amd64.deb
	ossctl cp dist/sichek_0.0.0~${VERSION}_linux_amd64.tar.gz scitix_oss/hisys-sichek/dev/0.0.0~${VERSION}/sichek_0.0.0~${VERSION}_linux_amd64.tar.gz

docker:
	docker build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg SICL_PKG_VERSION=${SICL_PKG_VERSION} \
	--build-arg SICL_PKG_NAME=${SICL_PKG_NAME} \
	-t registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION} -f docker/Dockerfile .
	docker push registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION}

sichek:
	BUILD_TIME=${BUILD_TIME} \
	INCLUDE_SICL=1 SICL_PKG_VERSION=${SICL_PKG_VERSION} \
	SICL_PKG_NAME=${SICL_PKG_NAME} \
	goreleaser release --snapshot --clean
	ossctl cp dist/sichek_0.0.0~${VERSION}_linux_amd64.deb scitix_oss/hisys-sichek/dev/0.0.0~${VERSION}/sichek_0.0.0~${VERSION}_linux_amd64.deb
	ossctl cp dist/sichek_0.0.0~${VERSION}_linux_amd64.tar.gz scitix_oss/hisys-sichek/dev/0.0.0~${VERSION}/sichek_0.0.0~${VERSION}_linux_amd64.tar.gz
	docker build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg SICL_PKG_VERSION=${SICL_PKG_VERSION} \
	--build-arg SICL_PKG_NAME=${SICL_PKG_NAME} \
	-t registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION} -f docker/Dockerfile .
	docker push registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION}

taskguard:
	docker build \
	-t registry-ap-southeast.scitix.ai/hisys/taskguard:${TASKGUARD_VERISON} \
	-f examples/taskguard/Dockerfile examples/taskguard
	docker push registry-ap-southeast.scitix.ai/hisys/taskguard:${TASKGUARD_VERISON}

pkg:
	mkdir -p ./dist
	docker build --target build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	-t sichek-build:${VERSION} -f docker/Dockerfile .
	docker create --name sichek-tmp-${VERSION} sichek-build:${VERSION}
	docker cp sichek-tmp-${VERSION}:/go/src/sichek/dist/. ./dist/
	docker rm sichek-tmp-${VERSION}

pkg-cuda130:
	mkdir -p ./dist-cuda130
	docker build --target build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	-t sichek-cuda130-build:${VERSION} -f docker/Dockerfile.cuda130 .
	docker create --name sichek-tmp-cuda130-${VERSION} sichek-cuda130-build:${VERSION}
	docker cp sichek-tmp-cuda130-${VERSION}:/go/src/sichek/dist/. ./dist-cuda130/
	docker rm sichek-tmp-cuda130-${VERSION}

pkg-cuda128:
	mkdir -p ./dist-cuda128
	docker build --target build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	-t sichek-cuda128-build:${VERSION} -f docker/Dockerfile.cuda128 .
	docker create --name sichek-tmp-cuda128-${VERSION} sichek-cuda128-build:${VERSION}
	docker cp sichek-tmp-cuda128-${VERSION}:/go/src/sichek/dist/. ./dist-cuda128/
	docker rm sichek-tmp-cuda128-${VERSION}

pkg-centos8:
	mkdir -p ./dist-centos8
	docker build --target build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	-t sichek-centos8-build:${VERSION} -f docker/Dockerfile.centos8 .
	docker create --name sichek-tmp-centos8-${VERSION} sichek-centos8-build:${VERSION}
	docker cp sichek-tmp-centos8-${VERSION}:/go/src/sichek/dist/. ./dist-centos8/
	docker rm sichek-tmp-centos8-${VERSION}

clean:
	rm -f build/bin/*
