package dockerdiscovery

import (
	"github.com/coredns/coredns/plugin"
	dockerapi "github.com/fsouza/go-dockerclient"
	"fmt"
	"net"
	"context"
	"github.com/miekg/dns"
	"github.com/coredns/coredns/request"
	"log"
	"strings"
	"errors"
)

type ContainerInfo struct {
	container *dockerapi.Container
	address   net.IP
	domains   []string // resolved domain
}

type ContainerInfoMap map[string]*ContainerInfo

type ContainerDomainResolver interface {
	// return domains without trailing dot
	resolve(container *dockerapi.Container) ([]string, error)
}

// DockerDiscovery is a plugin that conforms to the coredns plugin interface
type DockerDiscovery struct {
	Next           plugin.Handler
	dockerEndpoint string
	resolvers 	   []ContainerDomainResolver
	dockerClient   *dockerapi.Client
	containerInfoMap   ContainerInfoMap
	domainIPMap 	map[string]*net.IP
}
// NewDockerDiscovery constructs a new DockerDiscovery object
func NewDockerDiscovery(dockerEndpoint string) DockerDiscovery {
	return DockerDiscovery{
		dockerEndpoint: dockerEndpoint,
		containerInfoMap:   make(ContainerInfoMap),
	}
}

func (dd DockerDiscovery) resolveDomainsByContainer(container *dockerapi.Container) ([]string, error) {
	var domains []string
	for _, resolver := range dd.resolvers {
		var d, err = resolver.resolve(container)
		if err != nil {
			log.Printf("[docker] Error resolving container domains %s", err)
		}
		domains = append(domains, d...)
	}

	return domains, nil
}

func (dd DockerDiscovery) containerInfoByDomain(requestName string) (*ContainerInfo, error) {
	for _, containerInfo := range dd.containerInfoMap {
		for _, d := range containerInfo.domains {
			if fmt.Sprintf("%s.", d) == requestName { // qualified domain name must be specified with a trailing dot
				return containerInfo, nil
			}
		}
	}

	return nil, nil
}

// ServeDNS implements plugin.Handler
func (dd DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	var answers []dns.RR
	switch state.QType() {
	case dns.TypeA:
		containerInfo, _ := dd.containerInfoByDomain(state.QName())
		if containerInfo != nil {
			log.Printf("[docker] Found ip %v for host %s", containerInfo.address, state.QName())
			answers = a(state.Name(), []net.IP{containerInfo.address})
		}
	}

	if len(answers) == 0 {
		return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, true, true
	m.Answer = answers

	state.SizeAndDo(m)
	m = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements plugin.Handler
func (dd DockerDiscovery) Name() string {
	return "docker"
}

func (dd DockerDiscovery) getContainerAddress(container *dockerapi.Container) (net.IP, error) {
	for {
		if container.NetworkSettings.IPAddress != "" {
			return net.ParseIP(container.NetworkSettings.IPAddress), nil
		}

		networkMode := container.HostConfig.NetworkMode

		// TODO: Deal with containers run with host ip (--net=host)
		// if networkMode == "host" {
		// 	log.Println("[docker] Container uses host network")
		// 	return nil, nil
		// }

		if strings.HasPrefix(networkMode, "container:") {
			log.Printf("Container %s is in another container's network namspace", container.ID[:12])
			otherID := container.HostConfig.NetworkMode[len("container:")]
			var err error
			container, err = dd.dockerClient.InspectContainer(string(otherID))
			if err != nil {
				return nil, err
			}
			continue
		} else {
			network, ok := container.NetworkSettings.Networks[networkMode]
			if !ok { // sometime while "network:disconnect" event fire
				return nil, fmt.Errorf("unable to find network settings for the network %s", networkMode)
			}

			return net.ParseIP(network.IPAddress), nil  // ParseIP return nil when IPAddress equals ""
		}
	}
}

func (dd DockerDiscovery) updateContainerInfo(container *dockerapi.Container) error {
	_, isExist := dd.containerInfoMap[container.ID]
	containerAddress, err := dd.getContainerAddress(container)
	if isExist { // remove previous resolved container info
		delete(dd.containerInfoMap, container.ID)
	}

	if err != nil || containerAddress == nil {
		log.Printf("[docker] Remove container entry %s (%s)", container.Name, container.ID[:12])
		return err
	}

	domains, _ := dd.resolveDomainsByContainer(container)
	if len(domains) > 0 {
		dd.containerInfoMap[container.ID] = &ContainerInfo{
			container: container,
			address: containerAddress,
			domains: domains,
		}

		if !isExist {
			log.Printf("[docker] Add entry of container %s (%s). IP: %v", container.Name, container.ID[:12], containerAddress)
		}
	} else if isExist {
		log.Printf("[docker] Remove container entry %s (%s)", container.Name, container.ID[:12])
	}
	return nil
}

func (dd DockerDiscovery) removeContainerInfo(containerID string) error {
	containerInfo, ok := dd.containerInfoMap[containerID]
	if !ok {
		log.Printf("[docker] No entry associated with the container %s", containerID[:12])
		return nil
	}
	log.Printf("[docker] Deleting entry %s (%s)", containerInfo.container.Name, containerInfo.container.ID[:12])
	delete(dd.containerInfoMap, containerID)

	return nil
}

func (dd DockerDiscovery) start() error {
	log.Println("[docker] start")
	events := make(chan *dockerapi.APIEvents)

	if err := dd.dockerClient.AddEventListener(events); err != nil {
		return err
	}

	containers, err := dd.dockerClient.ListContainers(dockerapi.ListContainersOptions{})
	if err != nil {
		return err
	}

	for _, apiContainer := range containers {
		container, err := dd.dockerClient.InspectContainer(apiContainer.ID)
		if err != nil {
			// TODO err
		}
		if err := dd.updateContainerInfo(container); err != nil {
			log.Printf("[docker] Error adding A record for container %s: %s\n", container.ID[:12], err)
		}
	}

	for msg := range events {
		go func(msg *dockerapi.APIEvents) {
			event := fmt.Sprintf("%s:%s", msg.Type, msg.Action)
			switch event {
			case "container:start":
				log.Println("[docker] New container spawned. Attempt to add A record for it")

				container, err := dd.dockerClient.InspectContainer(msg.Actor.ID)
				if err != nil {
					log.Printf("[docker] Event error %s #%s: %s", event, msg.Actor.ID[:12], err)
					return
				}
				if err := dd.updateContainerInfo(container); err != nil {
					log.Printf("[docker] Error adding A record for container %s: %s", container.ID[:12], err)
				}
			case "container:die":
				log.Println("[docker] Container being stopped. Attempt to remove its A record from the DNS", msg.Actor.ID[:12])
				if err := dd.removeContainerInfo(msg.Actor.ID); err != nil {
					log.Printf("[docker] Error deleting A record for container: %s: %s", msg.Actor.ID[:12], err)
				}
			case "network:connect":
				// take a look https://gist.github.com/josefkarasek/be9bac36921f7bc9a61df23451594fbf for example of same event's types attributes
				log.Printf("[docker] Container %s being connected to network %s.", msg.Actor.Attributes["container"][:12], msg.Actor.Attributes["name"])

				container, err := dd.dockerClient.InspectContainer(msg.Actor.Attributes["container"])
				if err != nil {
					log.Printf("[docker] Event error %s #%s: %s", event, msg.Actor.Attributes["container"][:12], err)
					return
				}
				if err := dd.updateContainerInfo(container); err != nil {
					log.Printf("[docker] Error adding A record for container %s: %s", container.ID[:12], err)
				}
			case "network:disconnect":
				log.Printf("[docker] Container %s being disconnected from network %s", msg.Actor.Attributes["container"][:12], msg.Actor.Attributes["name"])

				container, err := dd.dockerClient.InspectContainer(msg.Actor.Attributes["container"])
				if err != nil {
					log.Printf("[docker] Event error %s #%s: %s", event, msg.Actor.Attributes["container"][:12], err)
					return
				}
				if err := dd.updateContainerInfo(container); err != nil {
					log.Printf("[docker] Error adding A record for container %s: %s", container.ID[:12], err)
				}
			}
		}(msg)
	}

	return errors.New("docker event loop closed")
}

// a takes a slice of net.IPs and returns a slice of A RRs.
func a(zone string, ips []net.IP) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.A)
		r.Hdr = dns.RR_Header{
			Name:   zone,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    3600,
		}
		r.A = ip
		answers = append(answers, r)
	}
	return answers
}
