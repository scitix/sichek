ARG CUDA_VERSION=12.8.1
FROM nvidia/cuda:${CUDA_VERSION}-base-ubuntu20.04 AS build

ARG GO_VERSION=1.23.3
ARG GORELEASER_VERSION=v2.13.1

RUN mv /etc/apt/sources.list.d/cuda*.list /tmp/disabled-cuda.list 2>/dev/null || true && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential gcc g++ curl git wget ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Install Go
RUN curl -fsSL https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz | \
    tar -C /usr/local -xz && \
    ln -s /usr/local/go/bin/go /usr/local/bin/go

# Install GoReleaser
RUN curl -fsSL https://github.com/goreleaser/goreleaser/releases/download/${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz | \
    tar -xz -C /usr/local/bin goreleaser && \
    goreleaser --version

ENV GOSUMDB=off
WORKDIR /go/src/sichek

COPY . .

RUN make goreleaser

FROM nvidia/cuda:${CUDA_VERSION}-base-ubuntu20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Asia/Shanghai

RUN mv /etc/apt/sources.list.d/cuda*.list /tmp/disabled-cuda.list 2>/dev/null || true && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        vim curl tzdata ca-certificates \
        openssh-server openssh-client \
        iproute2 iputils-ping jq \
        libnuma1 numactl rdmacm-utils perftest xz-utils && \
    ln -fs /usr/share/zoneinfo/${TZ} /etc/localtime && \
    dpkg-reconfigure --frontend noninteractive tzdata && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Helm (runtime tool)
RUN curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

WORKDIR /opt/sichek
COPY --from=build /go/src/sichek/dist ./dist

ARG SICL_VERSION=sicl-25.11-1.cuda128.ubuntu2004.run
RUN curl -fsSL -o ./dist/${SICL_VERSION} \
      https://oss-ap-southeast.scitix.ai/scitix-release/${SICL_VERSION} && \
    bash ./dist/${SICL_VERSION} && \
    dpkg -i ./dist/sichek_*_linux_amd64.deb

ENV SIHPC_HOME=/usr/local/sihpc
ENV PATH=${SIHPC_HOME}/bin:$PATH
ENV LD_LIBRARY_PATH=${SIHPC_HOME}/lib:/usr/local/cuda/lib64:$LD_LIBRARY_PATH
ENV OMPI_MCA_opal_prefix=${SIHPC_HOME}
ENV OPAL_PREFIX=${SIHPC_HOME}

# SSH dependencies for MPI
RUN sed -i 's/[ #]\(.*StrictHostKeyChecking \).*/ \1no/g' /etc/ssh/ssh_config && \
    echo "    UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config && \
    sed -i 's/#\(StrictModes \).*/\1no/g' /etc/ssh/sshd_config && \
    mkdir -p /var/run/sshd /root/.ssh && \
    chmod 700 /root/.ssh
RUN ssh-keygen -t rsa -f /root/.ssh/id_rsa -q -N "" && \
    cat /root/.ssh/id_rsa.pub >> /root/.ssh/authorized_keys && \
    chmod 600 /root/.ssh/authorized_keys

EXPOSE 22
