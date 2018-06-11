FROM golang:1.10 AS build
ADD . /go/src/dm-linuxkit
WORKDIR /go/src/dm-linuxkit
ENV VERSION=unversioned-FIXME
RUN go install -ldflags "-X main.serverVersion=${VERSION}"

FROM quay.io/coreos/etcd:v3.3 AS etcd

FROM quay.io/dotmesh/dotmesh-server:eda2009545b837fe785ea134653580f81cacdf81
COPY --from=build /go/bin/dm-linuxkit /usr/local/bin/dm-linuxkit
COPY --from=etcd /usr/local/bin/etcd /usr/local/bin/etcd
