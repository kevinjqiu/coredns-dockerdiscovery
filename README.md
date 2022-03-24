coredns-dockerdiscovery
===================================

Docker discovery plugin for coredns

Name
----

dockerdiscovery - add/remove DNS records for docker containers.

Syntax
------

    docker [DOCKER_ENDPOINT] {
        domain DOMAIN_NAME
        hostname_domain HOSTNAME_DOMAIN_NAME
        network_aliases DOCKER_NETWORK
        label LABEL
        compose_domain COMPOSE_DOMAIN_NAME
    }

* `DOCKER_ENDPOINT`: the path to the docker socket. If unspecified, defaults to `unix:///var/run/docker.sock`. It can also be TCP socket, such as `tcp://127.0.0.1:999`.
* `DOMAIN_NAME`: the name of the domain for [container name](https://docs.docker.com/engine/reference/run/#name---name), e.g. when `DOMAIN_NAME` is `docker.loc`, your container with `my-nginx` (as subdomain) [name](https://docs.docker.com/engine/reference/run/#name---name) will be assigned the domain name: `my-nginx.docker.loc`
* `HOSTNAME_DOMAIN_NAME`: the name of the domain for [hostname](https://docs.docker.com/config/containers/container-networking/#ip-address-and-hostname). Work same as `DOMAIN_NAME` for hostname.
* `COMPOSE_DOMAIN_NAME`: the name of the domain when it is determined the
    container is managed by docker-compose.  e.g. for a compose project of
    "internal" and service of "nginx", if `COMPOSE_DOMAIN_NAME` is
    `compose.loc` the fqdn will be `nginx.internal.compose.loc`
* `DOCKER_NETWORK`: the name of the docker network. Resolve directly by [network aliases](https://docs.docker.com/v17.09/engine/userguide/networking/configure-dns) (like internal docker dns resolve host by aliases whole network)
* `LABEL`: container label of resolving host (by default enable and equals ```coredns.dockerdiscovery.host```)

How To Build
------------

    GO111MODULE=on go get -u github.com/coredns/coredns
    GO111MODULE=on go get github.com/kevinjqiu/coredns-dockerdiscovery
    cd ~/go/src/github.com/coredns/coredns
    echo "docker:github.com/kevinjqiu/coredns-dockerdiscovery" >> plugin.cfg
    cat plugin.cfg | uniq > plugin.cfg.tmp
    mv plugin.cfg.tmp plugin.cfg
    make all
    ~/go/src/github.com/coredns/coredns/coredns --version

Alternatively, you can use the following manual steps:

1. Checkout coredns:  `go get github.com/coredns/coredns`.
2. `cd $GOPATH/src/github.com/coredns/coredns`
3. `echo "docker:github.com/kevinjqiu/coredns-dockerdiscovery" >> plugin.cfg`
4. `go generate`
5. `make`

Alternatively, run insider docker container

    docker build -t coredns-dockerdiscovery .
    docker run --rm -v ${PWD}/Corefile:/etc/Corefile -v /var/run/docker.sock:/var/run/docker.sock -p 15353:15353/udp coredns-dockerdiscovery -conf /etc/Corefile

Run tests

    go test -v

Example
-------

`Corefile`:

    .:15353 {
        docker unix:///var/run/docker.sock {
            domain docker.loc
            hostname_domain docker-host.loc
        }
        log
    }

Start CoreDNS:

    $ ./coredns

    .:15353
    2018/04/26 22:36:32 [docker] start
    2018/04/26 22:36:32 [INFO] CoreDNS-1.1.1
    2018/04/26 22:36:32 [INFO] linux/amd64, go1.10.1,
    CoreDNS-1.1.1

Start a docker container:

    $ docker run -d --name my-alpine --hostname alpine alpine sleep 1000
    78c2a06ef2a9b63df857b7985468f7310bba0d9ea4d0d2629343aff4fd171861

Use CoreDNS as your resolver to resolve the `my-alpine.docker.loc` or `alpine.docker-host.loc`:

    $ dig @localhost -p 15353 my-alpine.docker.loc

    ; <<>> DiG 9.10.3-P4-Ubuntu <<>> @localhost -p 15353 my-alpine.docker.loc
    ; (1 server found)
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 61786
    ;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

    ;; OPT PSEUDOSECTION:
    ; EDNS: version: 0, flags:; udp: 4096
    ;; QUESTION SECTION:
    ;my-alpine.docker.loc.            IN      A

    ;; ANSWER SECTION:
    my-alpine.docker.loc.     3600    IN      A       172.17.0.2

    ;; Query time: 0 msec
    ;; SERVER: 127.0.0.1#15353(127.0.0.1)
    ;; WHEN: Thu Apr 26 22:39:55 EDT 2018
    ;; MSG SIZE  rcvd: 63

Stop the docker container will remove the corresponded DNS entries:

    $ docker stop my-alpine
    78c2a

    $ dig @localhost -p 15353 my-alpine.docker.loc

    ;; QUESTION SECTION:
    ;my-alpine.docker.loc.            IN      A

Container will be resolved by label as ```nginx.loc```

    docker run --label=coredns.dockerdiscovery.host=nginx.loc nginx


 See receipt [how install for local development](setup.md)
