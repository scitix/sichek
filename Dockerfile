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

FROM nvidia/cuda:12.2.2-cudnn8-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    vim build-essential tzdata curl git sudo ca-certificates libgcc1 \
    && rm -rf /var/lib/apt/lists/*

ENV TZ=UTC
RUN ln -fs /usr/share/zoneinfo/$TZ /etc/localtime && dpkg-reconfigure --frontend noninteractive tzdata

WORKDIR /opt/sichek

# COPY --from=build /go/src/sichek/components/cpu/config/*.yaml /var/sichek/cpu/
# COPY --from=build /go/src/sichek/components/disk/config/*.yaml /var/sichek/disk/
# COPY --from=build /go/src/sichek/components/dmesg/config/*.yaml /var/sichek/dmesg/
# COPY --from=build /go/src/sichek/components/gpfs/config/*.yaml /var/sichek/gpfs/
# COPY --from=build /go/src/sichek/components/hang/config/*.yaml /var/sichek/hang/
# COPY --from=build /go/src/sichek/components/infiniband/config/*.yaml /var/sichek/infiniband/
# COPY --from=build /go/src/sichek/components/memory/config/*.yaml /var/sichek/memory/
# COPY --from=build /go/src/sichek/components/nccl/config/*.yaml /var/sichek/nccl/
# COPY --from=build /go/src/sichek/components/nvidia/config/*.yaml /var/sichek/nvidia/
# COPY --from=build /go/src/sichek/build/bin/* /usr/sbin/
COPY --from=build /go/src/sichek/dist/sichek_linux_amd64.deb .
RUN dpkg -i sichek_linux_amd64.deb && rm -rf sichek_linux_amd64.deb

EXPOSE 8080
