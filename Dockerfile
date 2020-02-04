FROM golang:1.13.7-stretch

ENV GO111MODULE=on

RUN mkdir -p $GOPATH/src/github.com/kevinjqiu/coredns-dockerdiscovery
COPY . /tmp/coredns-dockerdiscovery
RUN mv /tmp/coredns-dockerdiscovery/* $GOPATH/src/github.com/kevinjqiu/coredns-dockerdiscovery
RUN cd $GOPATH/src/github.com/kevinjqiu/coredns-dockerdiscovery && go mod download

WORKDIR $GOPATH/pkg/mod/github.com/coredns/coredns@v1.6.7
RUN echo "docker:github.com/kevinjqiu/coredns-dockerdiscovery" >> plugin.cfg
# avoid error build github.com/coredns/coredns: cannot load github.com/Azure/go-autorest/autorest: ambiguous import: found github.com/Azure/go-autorest/autorest in multiple modules
RUN sed -i '/azure:azure/d' plugin.cfg
RUN go generate coredns.go && go build

ENTRYPOINT ["./coredns"]