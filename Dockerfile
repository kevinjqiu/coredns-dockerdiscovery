FROM golang:1.13.5-stretch

#RUN apt-get update && apt-get -uy upgrade
#RUN apt-get -y install ca-certificates && update-ca-certificates

ENV GO111MODULE=on
#RUN go get github.com/coredns/coredns@v1.6.6
RUN mkdir -p $(go env GOPATH)/src/github.com/coredns/coredns && git clone https://github.com/coredns/coredns.git $(go env GOPATH)/src/github.com/coredns/coredns
# RUN mkdir -p $(go env GOPATH)/src/github.com/kevinjqiu/coredns-dockerdiscovery
COPY . $(go env GOPATH)/src/github.com/kevinjqiu/coredns-dockerdiscovery
RUN cd $(go env GOPATH)/src/github.com/coredns/coredns && echo "docker:github.com/kevinjqiu/coredns-dockerdiscovery" >> plugin.cfg && make coredns
RUN cp $(go env GOPATH)/src/github.com/coredns/coredns/coredns /coredns

EXPOSE 15353 15353/udp
ENTRYPOINT ["/coredns"]