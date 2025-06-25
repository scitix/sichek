.PHONY: docker  
PROJECT_NAME := sichek
GO := go
INSTALL_DIR := /usr/local/bin
VERSION_MAJOR := 0
VERSION_MINOR := 4
VERSION_PATCH := 3
GIT_COMMIT := $(shell git rev-parse --short HEAD)
GO_VERSION := $(shell $(GO) version | cut -d ' ' -f 3)
BUILD_TIME := $(shell date -u '+%Y-%m-%d')
VERSION:=v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH)
LDFLAGS := -X 'cmd/command/version.Major=$(VERSION_MAJOR)' \
           -X 'cmd/command/version.Minor=$(VERSION_MINOR)' \
           -X 'cmd/command/version.Patch=$(VERSION_PATCH)' \
           -X 'cmd/command/version.GitCommit=$(GIT_COMMIT)' \
           -X 'cmd/command/version.GoVersion=$(GO_VERSION)' \
           -X 'cmd/command/version.BuildTime=$(BUILD_TIME)'
TASKGUARD_VERISON := v0.1.0

all:
	mkdir -p build/bin/
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -gcflags "all=-N -l" -o build/bin/$(PROJECT_NAME) cmd/main.go

debug:
	mkdir -p build/bin/
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -gcflags "all=-N -l" -o build/bin/$(PROJECT_NAME) cmd/main.go
	VERSION_MAJOR=${VERSION_MAJOR} VERSION_MINOR=${VERSION_MINOR} VERSION_PATCH=${VERSION_PATCH} \
	GIT_COMMIT=${GIT_COMMIT} GO_VERSION=${GO_VERSION} BUILD_TIME=${BUILD_TIME} INSTALL_DIR=${INSTALL_DIR} \
	goreleaser release --snapshot --clean
	curl -X PUT "https://oss-ap-southeast.scitix.ai/scitix-release/sichek/vdebug/sichek_vdebug_linux_amd64.rpm" --upload-file ./dist/sichek_${VERSION}_linux_amd64.rpm
	curl -X PUT "https://oss-ap-southeast.scitix.ai/scitix-release/sichek/vdebug/sichek_vdebug_linux_amd64.deb" --upload-file ./dist/sichek_${VERSION}_linux_amd64.deb

docker:
	VERSION_MAJOR=${VERSION_MAJOR} VERSION_MINOR=${VERSION_MINOR} VERSION_PATCH=${VERSION_PATCH} \
	GIT_COMMIT=${GIT_COMMIT} GO_VERSION=${GO_VERSION} BUILD_TIME=${BUILD_TIME}  INSTALL_DIR=${INSTALL_DIR} \
	goreleaser release --snapshot --clean
	docker build \
	--build-arg VERSION_MAJOR=${VERSION_MAJOR} \
	--build-arg VERSION_MINOR=${VERSION_MINOR} \
	--build-arg VERSION_PATCH=${VERSION_PATCH} \
	--build-arg GIT_COMMIT=${GIT_COMMIT} \
	--build-arg GO_VERSION=${GO_VERSION} \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg INSTALL_DIR=${INSTALL_DIR} \
	-t registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION} -f docker/Dockerfile .
	docker push registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION}

release:
	VERSION_MAJOR=${VERSION_MAJOR} VERSION_MINOR=${VERSION_MINOR} VERSION_PATCH=${VERSION_PATCH} \
	GIT_COMMIT=${GIT_COMMIT} GO_VERSION=${GO_VERSION} BUILD_TIME=${BUILD_TIME} INSTALL_DIR=${INSTALL_DIR} \
	goreleaser release --snapshot --clean
	curl -X PUT "https://oss-ap-southeast.scitix.ai/scitix-release/sichek/latest/sichek_latest_linux_amd64.rpm" --upload-file ./dist/sichek_${VERSION}_linux_amd64.rpm
	curl -X PUT "https://oss-ap-southeast.scitix.ai/scitix-release/sichek/latest/sichek_latest_linux_amd64.deb" --upload-file ./dist/sichek_${VERSION}_linux_amd64.deb
	curl -X PUT "https://oss-ap-southeast.scitix.ai/scitix-release/sichek/${VERSION}/sichek_${VERSION}_linux_amd64.rpm" --upload-file ./dist/sichek_${VERSION}_linux_amd64.rpm
	curl -X PUT "https://oss-ap-southeast.scitix.ai/scitix-release/sichek/${VERSION}/sichek_${VERSION}_linux_amd64.deb" --upload-file ./dist/sichek_${VERSION}_linux_amd64.deb
	curl -X PUT "https://oss-ap-southeast.scitix.ai/scitix-release/sichek/install.sh" --upload-file ./install.sh
	docker build \
	--build-arg VERSION_MAJOR=${VERSION_MAJOR} \
	--build-arg VERSION_MINOR=${VERSION_MINOR} \
	--build-arg VERSION_PATCH=${VERSION_PATCH} \
	--build-arg GIT_COMMIT=${GIT_COMMIT} \
	--build-arg GO_VERSION=${GO_VERSION} \
	--build-arg BUILD_TIME=${BUILD_TIME} \
	--build-arg INSTALL_DIR=${INSTALL_DIR} \
	-t registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION} -f docker/Dockerfile .
	docker push registry-ap-southeast.scitix.ai/hisys/sichek:${VERSION}

taskguard:
	docker build \
	-t registry-ap-southeast.scitix.ai/hisys/taskguard:${TASKGUARD_VERISON} \
	-f examples/taskguard/Dockerfile examples/taskguard
	docker push registry-ap-southeast.scitix.ai/hisys/taskguard:${TASKGUARD_VERISON}

clean:
	rm -f build/bin/*

install: all
	# Install the binary to the specified directory
	cp build/bin/$(PROJECT_NAME) $(INSTALL_DIR)/$(PROJECT_NAME)
	@echo "Installed $(PROJECT_NAME) to $(INSTALL_DIR)/$(PROJECT_NAME)"