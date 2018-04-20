coredns plugin for docker discovery
===================================

Name
----

dockerdiscovery - add/remove DNS records for docker containers.

Syntax
------

    docker [DOCKER_SOCKET_PATH] {
        domain DOMAIN_NAME
    }


* `DOCKER_SOCKET_PATH`: the path to the docker socket. If unspecified, defaults to `/var/run/docker.sock`.
* `DOMAIN_NAME`: the name of the domain you want your containers to be part of. e.g., when `DOMAIN_NAME` is `docker.local`, your `mysql-0` container will be assigned the domain name: `mysql-0.docker.local`.

Example
-------

    .:15353 {
        docker /var/run/docker.sock {
            domain docker.local.
        }
    }
