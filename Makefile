test:
	go test -v

clean:
	rm -f coredns

coredns:
	go get -u github.com/fsouza/go-dockerclient
	go get -u github.com/coredns/coredns
	cd $(GOPATH)/src/github.com/coredns/coredns \
		&& echo "docker:github.com/kevinjqiu/coredns-dockerdiscovery" >> plugin.cfg \
		&& cat plugin.cfg | uniq > plugin.cfg.tmp \
		&& mv plugin.cfg.tmp plugin.cfg \
		&& go generate \
		&& make all
	cp $(GOPATH)/src/github.com/coredns/coredns/coredns coredns
