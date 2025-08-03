FROM nvidia/cuda:12.9.1-base-ubuntu22.04 AS build

RUN mv /etc/apt/sources.list.d/cuda*.list /tmp/disabled-cuda.list

RUN apt-get -o Acquire::http::No-Cache=true update && apt-get install -y \
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
# RUN go mod download

RUN make goreleaser

FROM nvidia/cuda:12.9.1-base-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN mv /etc/apt/sources.list.d/cuda*.list /tmp/disabled-cuda.list

RUN apt-get -o Acquire::http::No-Cache=true update && \
    apt-get install -y vim curl tzdata git ca-certificates perftest xz-utils \
    openssh-server libnuma1 numactl rdmacm-utils \
    openssh-client iproute2 iputils-ping jq && \
    rm -rf /var/lib/apt/lists/* && \
    curl -o /tmp/sicl_install.sh https://oss-ap-southeast.scitix.ai/scitix-release/sicl-24.11-1.cuda1262.ubuntu2204.run && \
    bash /tmp/sicl_install.sh && rm -rf /tmp/sicl_install.sh && \
    apt clean && apt autoremove && rm -rf /var/lib/apt/lists/*

RUN cd /tmp && \
    curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 && \
    chmod 700 get_helm.sh && ./get_helm.sh && rm -rf /tmp/get_helm.sh
    
RUN cd /tmp && \
    curl -o nccl-test-portable-2.27.3.tar.gz https://oss-ap-southeast.scitix.ai/scitix-release/nccl-test-portable/nccl-test-portable-2.27.3.tar.gz && \
    tar -xzf nccl-test-portable-2.27.3.tar.gz && \
    cp nccl-test-portable/lib/*  /usr/local/sihpc/lib/ && \
    rm -rf /tmp/*

ENV NVIDIA_VISIBLE_DEVICES=""
ENV TZ=Asia/Shanghai
RUN ln -fs /usr/share/zoneinfo/$TZ /etc/localtime && dpkg-reconfigure --frontend noninteractive tzdata

WORKDIR /opt/sichek
COPY --from=build /go/src/sichek/dist ./dist
RUN dpkg -i ./dist/sichek_*_linux_amd64.deb

# SSH dependencies for MPI
RUN sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
    echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
    sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config && \
    mkdir -p /var/run/sshd && \
    mkdir -p /root/.ssh && \
    chmod 700 /root/.ssh
    RUN ssh-keygen -t rsa -f /root/.ssh/id_rsa -q -N "" && \
    cat /root/.ssh/id_rsa.pub >> /root/.ssh/authorized_keys && \
    chmod 600 /root/.ssh/authorized_keys

EXPOSE 22