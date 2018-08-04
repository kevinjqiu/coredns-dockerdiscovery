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
    }


* `DOCKER_ENDPOINT`: the path to the docker socket. If unspecified, defaults to `unix:///var/run/docker.sock`. It can also be TCP socket, such as `tcp://127.0.0.1:999`.
* `DOMAIN_NAME`: the name of the domain you want your containers to be part of. e.g., when `DOMAIN_NAME` is `docker.local`, your `mysql-0` container will be assigned the domain name: `mysql-0.docker.local`.

How To Build
------------

`make coredns`

Alternatively, you can use the following manual steps:

1. Checkout coredns:  `go get github.com/coredns/coredns`.
2. `cd $GOPATH/src/github.com/coredns/coredns`
3. `echo "docker:github.com/kevinjqiu/coredns-dockerdiscovery" >> plugin.cfg`
4. `go generate`
5. `make`

Example
-------

`Corefile`:

    .:15353 {
        docker unix:///var/run/docker.sock {
            domain docker.local.
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

    $ docker run -d --hostname alpha alpine sleep 1000
    78c2a06ef2a9b63df857b7985468f7310bba0d9ea4d0d2629343aff4fd171861

Use CoreDNS as your resolver to resovle the `alpha.docker.local`:

    $ dig @localhost -p 15353 alpha.docker.local

    ; <<>> DiG 9.10.3-P4-Ubuntu <<>> @localhost -p 15353 alpha.docker.local
    ; (1 server found)
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 61786
    ;; flags: qr aa rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

    ;; OPT PSEUDOSECTION:
    ; EDNS: version: 0, flags:; udp: 4096
    ;; QUESTION SECTION:
    ;alpha.docker.local.            IN      A

    ;; ANSWER SECTION:
    alpha.docker.local.     3600    IN      A       172.17.0.2

    ;; Query time: 0 msec
    ;; SERVER: 127.0.0.1#15353(127.0.0.1)
    ;; WHEN: Thu Apr 26 22:39:55 EDT 2018
    ;; MSG SIZE  rcvd: 63

Stop the docker container will remove the DNS entry for `alpha.docker.local`:

    $ docker stop 78c2a
    78c2a


    $ dig @localhost -p 15353 alpha.docker.local

    ; <<>> DiG 9.10.3-P4-Ubuntu <<>> @localhost -p 15353 alpha.docker.local
    ; (1 server found)
    ;; global options: +cmd
    ;; Got answer:
    ;; ->>HEADER<<- opcode: QUERY, status: SERVFAIL, id: 52639
    ;; flags: qr rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 1
    ;; WARNING: recursion requested but not available

    ;; OPT PSEUDOSECTION:
    ; EDNS: version: 0, flags:; udp: 4096
    ;; QUESTION SECTION:
    ;alpha.docker.local.            IN      A

    ;; Query time: 0 msec
    ;; SERVER: 127.0.0.1#15353(127.0.0.1)
    ;; WHEN: Thu Apr 26 22:41:38 EDT 2018
    ;; MSG SIZE  rcvd: 47

Alternatively, run insider docker container

    docker build -t coredns-dockerdiscovery .
    docker run --rm -v ${PWD}/../Corefile:/coredns-dockerdiscovery/Corefile -v /var/run/docker.sock:/var/run/docker.sock -p 15353:15353/udp coredns-dockerdiscovery -conf /coredns-dockerdiscovery/Corefile