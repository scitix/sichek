FROM golang:1.23.3 as base

ENV GO111MODULE=auto
ENV GOSUMDB=off
WORKDIR /go/src/taskguard
# cache deps
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/
# code
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY main.go main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -o bin/taskguard

FROM rockylinux:8

COPY etc/*.yaml /opt/taskguard/etc/
COPY --from=base /go/src/taskguard/bin/taskguard /opt/taskguard/taskguard

WORKDIR /opt/taskguard
ENTRYPOINT ["taskguard"]
