FROM golang:1.14.4-stretch

RUN go mod download github.com/coredns/coredns@v1.7.0

WORKDIR $GOPATH/pkg/mod/github.com/coredns/coredns@v1.7.0
RUN go mod download

RUN sed -i '50 i docker:github.com/kevinjqiu/coredns-dockerdiscovery' plugin.cfg
ENV CGO_ENABLED=0
RUN go generate coredns.go && go build -mod=mod -o=/usr/local/bin/coredns

FROM alpine:3.12.0

RUN apk --no-cache add ca-certificates
COPY --from=0 /usr/local/bin/coredns /usr/local/bin/coredns

ENTRYPOINT ["/usr/local/bin/coredns"]