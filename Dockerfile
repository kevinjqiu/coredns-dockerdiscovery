ARG GOLANG_VERS=1.18
ARG ALPINE_VERS=3.17

FROM golang:${GOLANG_VERS}-alpine${ALPINE_VERS}

ARG CGO_ENABLED=1
ARG PLUGIN_PRIO=50
ARG COREDNS_VERS=1.10.1

RUN go mod download github.com/coredns/coredns@v${COREDNS_VERS}
WORKDIR $GOPATH/pkg/mod/github.com/coredns/coredns@v${COREDNS_VERS}
RUN go mod download

COPY --link ./ $GOPATH/pkg/mod/github.com/kevinjqiu/coredns-dockerdiscovery
RUN sed -i "s/^#.*//g; /^$/d; $PLUGIN_PRIO i docker:dockerdiscovery" plugin.cfg \
    && go mod edit -replace\
    dockerdiscovery=$GOPATH/pkg/mod/github.com/kevinjqiu/coredns-dockerdiscovery\
    && go generate coredns.go && go build -mod=mod -o=/usr/local/bin/coredns && \
    apk --no-cache add binutils && strip -vs /usr/local/bin/coredns

FROM alpine:${ALPINE_VERS}
RUN apk --no-cache add ca-certificates
COPY --from=0 /usr/local/bin/coredns /usr/local/bin/coredns

ENTRYPOINT ["/usr/local/bin/coredns"]
