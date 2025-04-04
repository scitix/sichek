FROM nvidia/cuda:11.8.0-cudnn8-devel-ubuntu20.04 AS build

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
RUN goreleaser release --snapshot --clean

FROM nvidia/cuda:12.8.0-base-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    vim build-essential tzdata curl git sudo ca-certificates libgcc1 perftest\
    && rm -rf /var/lib/apt/lists/*

ENV TZ=UTC
RUN ln -fs /usr/share/zoneinfo/$TZ /etc/localtime && dpkg-reconfigure --frontend noninteractive tzdata

WORKDIR /opt/sichek

COPY --from=build /go/src/sichek/dist/sichek_linux_amd64.deb .
RUN dpkg -i sichek_linux_amd64.deb && rm -rf sichek_linux_amd64.deb

EXPOSE 8080
