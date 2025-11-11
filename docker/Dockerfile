FROM nvidia/cuda:12.8.1-base-ubuntu20.04 AS build

RUN mv /etc/apt/sources.list.d/cuda*.list /tmp/disabled-cuda.list

RUN apt-get -o Acquire::http::No-Cache=true update && apt-get install -y \
    build-essential gcc g++ curl git wget \
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

ARG VERSION
RUN INCLUDE_SICL=true make goreleaser
RUN set -ex && \
    PRIMARY_URL="https://oss-ap-southeast.scitix.ai/scitix/packages/ossctl/latest/linux-amd64/ossctl" && \
    BACKUP_URL="https://oss-cn-shanghai.siflow.cn/scitix/packages/ossctl/latest/linux-amd64/ossctl" && \
    \
    echo "Checking primary URL accessibility..." && \
    if wget -q --spider "$PRIMARY_URL"; then \
        echo "Primary URL available, using $PRIMARY_URL"; \
        wget "$PRIMARY_URL" -O ossctl; \
    else \
        echo "Primary URL unavailable, switching to backup $BACKUP_URL"; \
        wget "$BACKUP_URL" -O ossctl; \
    fi && \
    \
    chmod +x ossctl && \
    ./ossctl config import scitix_oss ./tools/ossctl/credentials.json && \
    ./ossctl cp ./dist/sichek_${VERSION}_linux_amd64.rpm scitix_oss/scitix-release/sichek/latest/sichek_latest_linux_amd64.rpm && \
    ./ossctl cp ./dist/sichek_${VERSION}_linux_amd64.deb scitix_oss/scitix-release/sichek/latest/sichek_latest_linux_amd64.deb && \
    ./ossctl cp ./dist/sichek_${VERSION}_linux_amd64.rpm scitix_oss/scitix-release/sichek/${VERSION}/sichek_${VERSION}_linux_amd64.rpm && \
    ./ossctl cp ./dist/sichek_${VERSION}_linux_amd64.deb scitix_oss/scitix-release/sichek/${VERSION}/sichek_${VERSION}_linux_amd64.deb && \
    ./ossctl cp ./install.sh scitix_oss/scitix-release/sichek/install.sh

RUN rm -rf ./dist && \
    INCLUDE_SICL=false make goreleaser

FROM nvidia/cuda:12.8.1-base-ubuntu20.04

ENV DEBIAN_FRONTEND=noninteractive

RUN mv /etc/apt/sources.list.d/cuda*.list /tmp/disabled-cuda.list

RUN apt-get -o Acquire::http::No-Cache=true update && \
    apt-get install -y vim curl tzdata git ca-certificates perftest xz-utils \
    openssh-server libnuma1 numactl rdmacm-utils \
    openssh-client iproute2 iputils-ping jq && \
    apt clean && apt autoremove && rm -rf /var/lib/apt/lists/*

RUN cd /tmp && \
    curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 && \
    chmod 700 get_helm.sh && ./get_helm.sh && rm -rf /tmp/get_helm.sh


ENV NVIDIA_VISIBLE_DEVICES=""
ENV TZ=Asia/Shanghai
RUN ln -fs /usr/share/zoneinfo/$TZ /etc/localtime && dpkg-reconfigure --frontend noninteractive tzdata

WORKDIR /opt/sichek
COPY --from=build /go/src/sichek/dist ./dist
ARG SICL_VERSION=sicl-25.11-1.cuda128.ubuntu2004.run
RUN dpkg -i ./dist/sichek_*_linux_amd64.deb && \
    curl -o ./dist/${SICL_VERSION} https://oss-ap-southeast.scitix.ai/scitix-release/${SICL_VERSION}
ENV SIHPC_HOME=/usr/local/sihpc
ENV PATH=$SIHPC_HOME/bin:$PATH
ENV LD_LIBRARY_PATH=$SIHPC_HOME/lib:/usr/local/cuda/lib64:$LD_LIBRARY_PATH
ENV OMPI_MCA_opal_prefix=$SIHPC_HOME
ENV OPAL_PREFIX=$SIHPC_HOME

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