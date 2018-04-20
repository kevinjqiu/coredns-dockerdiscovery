coredns plugin for docker discovery
===================================

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

Example
-------

    .:15353 {
        docker unix:///var/run/docker.sock {
            domain docker.local.
        }
    }
