.PHONY: docker
PROJECT_NAME := sichek
GO := go
INSTALL_DIR := /usr/local/bin

# Version variables (can be overridden)
GIT_COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y%m%dT%H%M%SZ')
SICHEK_VERSION = $(shell git tag --sort=-v:refname | head -n 1)
VERSION := $(SICHEK_VERSION)-dev-${BUILD_TIME}-${GIT_COMMIT}
TASKGUARD_VERISON := v0.1.0
SICL_VERSION := sicl-25.11-1.cuda128.ubuntu2004.run

all:
	mkdir -p build/bin/
	go mod tidy
	go mod vendor
	GOOS=linux GOARCH=amd64 $(GO) build -gcflags "all=-N -l" -o build/bin/$(PROJECT_NAME) cmd/main.go

goreleaser:
	BUILD_TIME=${BUILD_TIME} \
	INCLUDE_SICL=${INCLUDE_SICL} SICL_VERSION=${SICL_VERSION} \
	goreleaser release --snapshot --clean
	@echo "Uploading packages to siflow_oss/hisys-sichek/dev/"
	@for pattern in "deb" "rpm" "tar.gz"; do \
		file=$$(ls dist/*linux_amd64.$$pattern 2>/dev/null | head -n1); \
		if [ -n "$$file" ]; then \
			echo "Copying $$file"; \
			ossctl cp "$$file" siflow_oss/hisys-sichek/dev/; \
		else \
			echo "No $$pattern file found"; \
		fi; \
	done
	@echo "Package upload completed"

docker:
	docker build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg SICL_VERSION=${SICL_VERSION} \
	-t registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION} -f docker/Dockerfile .
	docker push registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION}

sichek:
	INCLUDE_SICL=true $(MAKE) goreleaser
	@echo "Uploading packages to scitix_oss/hisys-sichek/dev/"
	@for pattern in "deb" "rpm" "tar.gz"; do \
		file=$$(ls dist/*linux_amd64.$$pattern 2>/dev/null | head -n1); \
		if [ -n "$$file" ]; then \
			echo "Copying $$file"; \
			ossctl cp "$$file" scitix_oss/hisys-sichek/dev/${VERSION}/; \
		else \
			echo "No $$pattern file found"; \
		fi; \
	done
	@echo "Package upload completed"
	docker build \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg SICL_VERSION=${SICL_VERSION} \
	-t registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION} -f docker/Dockerfile .
	docker push registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION}

taskguard:
	docker build \
	-t registry-ap-southeast.scitix.ai/hisys/taskguard:${TASKGUARD_VERISON} \
	-f examples/taskguard/Dockerfile examples/taskguard
	docker push registry-ap-southeast.scitix.ai/hisys/taskguard:${TASKGUARD_VERISON}

clean:
	rm -f build/bin/*
	rm -rf dist/*
	rm -rf scitix_oss
