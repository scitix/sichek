FROM nvidia/cuda:12.2.2-base-ubuntu22.04 AS build

RUN apt-get update && apt-get install -y \
    build-essential gcc g++ curl git \
    && rm -rf /var/lib/apt/lists/*

RUN curl -OL https://go.dev/dl/go1.23.3.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.23.3.linux-amd64.tar.gz && \
    rm go1.23.3.linux-amd64.tar.gz && \
    ln -s /usr/local/go/bin/go /usr/bin/go

# Install GoReleaser
RUN curl -sL https://github.com/goreleaser/goreleaser/releases/latest/download/goreleaser_Linux_x86_64.tar.gz | tar xz -C /usr/local/bin && goreleaser --version

ENV GO111MODULE=auto
ENV GOSUMDB=off
WORKDIR /go/src/sichek
# cache deps
# COPY go.mod go.mod
# COPY go.sum go.sum
# code
COPY . .
RUN go mod download

ARG VERSION_MAJOR=0
ARG VERSION_MINOR=3
ARG VERSION_PATCH=1
ARG GIT_COMMIT="none"
ARG GO_VERSION="none"
ARG BUILD_TIME="unknow"
ARG INSTALL_DIR="/usr/local/bin"
RUN echo "VERSION_MAJOR=${VERSION_MAJOR}" && \
    echo "VERSION_MINOR=${VERSION_MINOR}" && \
    echo "VERSION_PATCH=${VERSION_PATCH}" && \
    echo "GIT_COMMIT=${GIT_COMMIT}" && \
    echo "GO_VERSION=${GO_VERSION}" && \
    echo "BUILD_TIME=${BUILD_TIME}" && \
    echo "INSTALL_DIR=${INSTALL_DIR}" && \
    VERSION_MAJOR=${VERSION_MAJOR} VERSION_MINOR=${VERSION_MINOR} VERSION_PATCH=${VERSION_PATCH} \
    GIT_COMMIT=${GIT_COMMIT} GO_VERSION=${GO_VERSION} BUILD_TIME=${BUILD_TIME} INSTALL_DIR=${INSTALL_DIR} \
    goreleaser release --snapshot --clean

FROM nvidia/cuda:12.2.2-base-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y vim curl tzdata git ca-certificates perftest xz-utils libnuma1 numactl && \
    rm -rf /var/lib/apt/lists/* && \
    curl -o /tmp/sicl_install.sh https://oss-ap-southeast.scitix.ai/scitix-release/sicl-24.11-1.cuda1262.ubuntu2204.run && \
    bash /tmp/sicl_install.sh && rm -rf /tmp/sicl_install.sh

ENV NVIDIA_VISIBLE_DEVICES=""
ENV TZ=UTC
RUN ln -fs /usr/share/zoneinfo/$TZ /etc/localtime && dpkg-reconfigure --frontend noninteractive tzdata

WORKDIR /opt/sichek

COPY --from=build /go/src/sichek/dist ./dist
RUN dpkg -i ./dist/sichek_*_linux_amd64.deb && rm -rf ./dist

EXPOSE 8080
