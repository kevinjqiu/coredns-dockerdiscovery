FROM golang:1.16.5-stretch

RUN go mod download github.com/coredns/coredns@v1.8.4

WORKDIR $GOPATH/pkg/mod/github.com/coredns/coredns@v1.8.4
RUN go mod download

RUN sed -i '50 i docker:github.com/kevinjqiu/coredns-dockerdiscovery' plugin.cfg
ENV CGO_ENABLED=0
RUN go generate coredns.go && go build -mod=mod -o=/usr/local/bin/coredns

FROM alpine:3.13.5

RUN apk --no-cache add ca-certificates
COPY --from=0 /usr/local/bin/coredns /usr/local/bin/coredns

ENTRYPOINT ["/usr/local/bin/coredns"]
