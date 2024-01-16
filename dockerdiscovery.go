package dockerdiscovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/miekg/dns"
)

type ContainerInfo struct {
	container *dockerapi.Container
	address   net.IP
	address6  net.IP
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
	resolvers      []ContainerDomainResolver
	dockerClient   *dockerapi.Client

	mutex            sync.RWMutex
	containerInfoMap ContainerInfoMap
	ttl              uint32
}

// NewDockerDiscovery constructs a new DockerDiscovery object
func NewDockerDiscovery(dockerEndpoint string) *DockerDiscovery {
	return &DockerDiscovery{
		dockerEndpoint:   dockerEndpoint,
		containerInfoMap: make(ContainerInfoMap),
		ttl:              3600,
	}
}

func (dd *DockerDiscovery) resolveDomainsByContainer(container *dockerapi.Container) ([]string, error) {
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

func (dd *DockerDiscovery) containerInfoByDomain(requestName string) (*ContainerInfo, error) {
	dd.mutex.RLock()
	defer dd.mutex.RUnlock()

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
func (dd *DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	var answers []dns.RR
	switch state.QType() {
	case dns.TypeA:
		containerInfo, _ := dd.containerInfoByDomain(state.QName())
		if containerInfo != nil {
			answers = getAnswer(state.Name(), []net.IP{containerInfo.address}, dd.ttl, false)
		}
	case dns.TypeAAAA:
		containerInfo, _ := dd.containerInfoByDomain(state.QName())
		if containerInfo != nil && containerInfo.address6 != nil {
			answers = getAnswer(state.Name(), []net.IP{containerInfo.address6}, dd.ttl, true)
		} else if containerInfo != nil && containerInfo.address != nil {
			// in acordance with https://tools.ietf.org/html/rfc6147#section-5.1.2 we should return an empty answer section if no AAAA records are available and a A record is available when the client requested AAAA
			record := new(dns.AAAA)
			record.Hdr = dns.RR_Header{
				Name:   state.Name(),
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    dd.ttl,
				Rdlength: 0,
			}
			answers = append(answers, record)
		}
	}

	if len(answers) == 0 {
		return plugin.NextOrFailure(dd.Name(), dd.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative, m.RecursionAvailable, m.Compress = true, false, true
	m.Answer = answers

	state.SizeAndDo(m)
	m = state.Scrub(m)
	err := w.WriteMsg(m)
	if err != nil {
		log.Printf("[docker] Error: %s", err.Error())
	}
	return dns.RcodeSuccess, nil
}

// Name implements plugin.Handler
func (dd *DockerDiscovery) Name() string {
	return "docker"
}

func (dd *DockerDiscovery) getContainerAddress(container *dockerapi.Container, v6 bool) (net.IP, error) {

	// save this away
	netName, hasNetName := container.Config.Labels["coredns.dockerdiscovery.network"]

	var networkMode string

	for {
		if container.NetworkSettings.IPAddress != "" && !hasNetName && !v6 {
			return net.ParseIP(container.NetworkSettings.IPAddress), nil
		}

		if container.NetworkSettings.GlobalIPv6Address != "" && !hasNetName && v6 {
			return net.ParseIP(container.NetworkSettings.GlobalIPv6Address), nil
		}

		networkMode = container.HostConfig.NetworkMode

		// TODO: Deal with containers run with host ip (--net=host)
		if networkMode == "host" {
			log.Println("[docker] Container uses host network")
			return nil, nil
		}

		if strings.HasPrefix(networkMode, "container:") {
			log.Printf("Container %s is in another container's network namspace", container.ID[:12])
			otherID := container.HostConfig.NetworkMode[len("container:"):]
			var err error
			container, err = dd.dockerClient.InspectContainerWithOptions(dockerapi.InspectContainerOptions{ID: otherID})
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}

	var (
		network dockerapi.ContainerNetwork
		ok      = false
	)

	if hasNetName {
		log.Printf("[docker] network name %s specified (%s)", netName, container.ID[:12])
		network, ok = container.NetworkSettings.Networks[netName]
	} else if len(container.NetworkSettings.Networks) == 1 {
		for netName, network = range container.NetworkSettings.Networks {
			ok = true
		}
	}

	if !ok { // sometime while "network:disconnect" event fire
		return nil, fmt.Errorf("unable to find network settings for the network %s", networkMode)
	}

	if !v6 {
		return net.ParseIP(network.IPAddress), nil // ParseIP return nil when IPAddress equals ""
	} else if v6 && len(network.GlobalIPv6Address) > 0 {
		return net.ParseIP(network.GlobalIPv6Address), nil
	}

	return nil, nil
}

func (dd *DockerDiscovery) updateContainerInfo(container *dockerapi.Container) error {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	_, isExist := dd.containerInfoMap[container.ID]
	if isExist { // remove previous resolved container info
		delete(dd.containerInfoMap, container.ID)
	}

	containerAddress, err := dd.getContainerAddress(container, false)
	if err != nil || containerAddress == nil {
		log.Printf("[docker] Remove container entry %s (%s)", normalizeContainerName(container), container.ID[:12])
		return err
	}

	containerAddress6, err := dd.getContainerAddress(container, true)

	domains, _ := dd.resolveDomainsByContainer(container)
	if len(domains) > 0 {
		dd.containerInfoMap[container.ID] = &ContainerInfo{
			container: container,
			address:   containerAddress,
			address6:  containerAddress6,
			domains:   domains,
		}

		if !isExist {
			log.Printf("[docker] Add entry of container %s (%s). IP: %v", normalizeContainerName(container), container.ID[:12], containerAddress)
		}
	} else if isExist {
		log.Printf("[docker] Remove container entry %s (%s)", normalizeContainerName(container), container.ID[:12])
	}
	return nil
}

func (dd *DockerDiscovery) removeContainerInfo(containerID string) error {
	dd.mutex.Lock()
	defer dd.mutex.Unlock()

	containerInfo, ok := dd.containerInfoMap[containerID]
	if !ok {
		log.Printf("[docker] No entry associated with the container %s", containerID[:12])
		return nil
	}
	log.Printf("[docker] Deleting entry %s (%s)", normalizeContainerName(containerInfo.container), containerInfo.container.ID[:12])
	delete(dd.containerInfoMap, containerID)

	return nil
}

func (dd *DockerDiscovery) start() error {
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
		container, err := dd.dockerClient.InspectContainerWithOptions(dockerapi.InspectContainerOptions{ID: apiContainer.ID})
		if err != nil {
			// TODO err
		}
		if err := dd.updateContainerInfo(container); err != nil {
			log.Printf("[docker] Error adding A/AAAA records for container %s: %s\n", container.ID[:12], err)
		}
	}

	for msg := range events {
		go func(msg *dockerapi.APIEvents) {
			event := fmt.Sprintf("%s:%s", msg.Type, msg.Action)
			switch event {
			case "container:start":
				log.Println("[docker] New container spawned. Attempt to add A/AAAA records for it")

				container, err := dd.dockerClient.InspectContainerWithOptions(dockerapi.InspectContainerOptions{ID: msg.Actor.ID})
				if err != nil {
					log.Printf("[docker] Event error %s #%s: %s", event, msg.Actor.ID[:12], err)
					return
				}
				if err := dd.updateContainerInfo(container); err != nil {
					log.Printf("[docker] Error adding A/AAAA records for container %s: %s", container.ID[:12], err)
				}
			case "container:die":
				log.Println("[docker] Container being stopped. Attempt to remove its A/AAAA records from the DNS", msg.Actor.ID[:12])
				if err := dd.removeContainerInfo(msg.Actor.ID); err != nil {
					log.Printf("[docker] Error deleting A/AAAA records for container: %s: %s", msg.Actor.ID[:12], err)
				}
			case "network:connect":
				// take a look https://gist.github.com/josefkarasek/be9bac36921f7bc9a61df23451594fbf for example of same event's types attributes
				log.Printf("[docker] Container %s being connected to network %s.", msg.Actor.Attributes["container"][:12], msg.Actor.Attributes["name"])

				container, err := dd.dockerClient.InspectContainerWithOptions(dockerapi.InspectContainerOptions{ID: msg.Actor.Attributes["container"]})
				if err != nil {
					log.Printf("[docker] Event error %s #%s: %s", event, msg.Actor.Attributes["container"][:12], err)
					return
				}
				if err := dd.updateContainerInfo(container); err != nil {
					log.Printf("[docker] Error adding A/AAAA records for container %s: %s", container.ID[:12], err)
				}
			case "network:disconnect":
				log.Printf("[docker] Container %s being disconnected from network %s", msg.Actor.Attributes["container"][:12], msg.Actor.Attributes["name"])

				container, err := dd.dockerClient.InspectContainerWithOptions(dockerapi.InspectContainerOptions{ID: msg.Actor.Attributes["container"]})
				if err != nil {
					log.Printf("[docker] Event error %s #%s: %s", event, msg.Actor.Attributes["container"][:12], err)
					return
				}
				if err := dd.updateContainerInfo(container); err != nil {
					log.Printf("[docker] Error adding A/AAAA records for container %s: %s", container.ID[:12], err)
				}
			}
		}(msg)
	}

	return errors.New("docker event loop closed")
}

// getAnswer function takes a slice of net.IPs and returns a slice of A/AAAA RRs.
func getAnswer(zone string, ips []net.IP, ttl uint32, v6 bool) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		if !v6 {
			record := new(dns.A)
			record.Hdr = dns.RR_Header{
				Name:   zone,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			}
			record.A = ip
			answers = append(answers, record)
		} else if v6 {
			record := new(dns.AAAA)
			record.Hdr = dns.RR_Header{
				Name:   zone,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			}
			record.AAAA = ip
			answers = append(answers, record)
		}
	}
	return answers
}
