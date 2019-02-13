Install for local development
---

Create `Corefile`
    
    loc:53 {
        docker unix:///var/run/docker.sock {
            domain docker.loc
        }
        cache 20
        log
    }

Run coredns in alpine container with assigned static ip. Specify any network is required.
`${PWD}/coredns` path to executed coredns file. See `How To Build` section in [README.md](README.md)

    docker run -v /var/run/docker.sock:/var/run/docker.sock -v ${PWD}/coredns:/coredns -v ${HOME}/Corefile:/Corefile --network=any_network --ip=172.19.5.5 --name=coredns --restart=unless-stopped -d alpine /coredns 

Note --ip  have `any_network` mask (in my case 172.19.0.0)
Run any container for test and check out to correct resolve of container (coredns container ip is specified)
    
    docker run --name my-rabbitmq rabbitmq 
    dig @172.19.5.5 my-rabbitmq.docker.loc
    
    ;; ANSWER SECTION:
    my-rabbitmq.docker.loc. 2335 IN A 172.17.0.2

Install [resolvconf](https://en.wikipedia.org/wiki/Resolvconf) packet if you don't have

Add coredns's container ip to resolve.conf
    
    echo "nameserver 172.19.5.5" | sudo tee --append /etc/resolvconf/resolv.conf.d/tail
    sudo resolvconf -u

Check container resolving

    dig my-rabbitmq.docker.loc
    my-rabbitmq.docker.loc. 2335 IN A 172.17.0.2

Open in your browser http://my-rabbitmq.docker.loc:15672 (rabbitmq dashboard work on 15672 by default)

Development with multiple docker-compose microservices
----

Create `Corefile`

    my-project.loc:15353 {
        docker unix:///var/run/docker.sock {
            network_aliases my_project_network
        }
        cache 20
        log
    }

Create `my_project_network` network
 
    docker create network my_project_network

Example `docker-compose.yml` for multiple services project.
Add external network `my_project_network` to every compose file.

Payment service:

    version: "3.7"

    services:
      nginx:
        image: nginx:latest
        networks:
          default:
            aliases:
              - payment.my-project.loc
    networks:
      default:
        external:
          name: my_project_network

Auth service:

    version: "3.7"
    
    services:
      nginx:
        image: nginx:latest
        networks:
          default:
            aliases:
              - auth.my-project.loc
    networks:
      default:
        external:
          name: my_project_network

Or run container with specify our network  

    docker run --network my_project_network --alias postgres.my-project.loc postgres
       
Check out access to container via local dns `payment.my-project.loc`, `auth.my-project.loc` and `postgres.my-project.loc`

    $ dig payment.my-project.loc
    
    ;; ANSWER SECTION:
    payment.my-project.loc. 2335 IN A 172.20.0.2

Our localhost and all containers in `my_project_network` network have access to each other by aliases.

It's let you reach any services without reverse proxy and avoid ports conflict, confused specified unique published ports.