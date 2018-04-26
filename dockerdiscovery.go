package dockerdiscovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/coredns/coredns/request"

	"github.com/coredns/coredns/plugin"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/miekg/dns"
)

type containerIPMap struct {
	containerIDHostNameMap map[string][]string
	byHostName             map[string]net.IP
}

// DockerDiscovery is a plugin that conforms to the coredns plugin interface
type DockerDiscovery struct {
	Next           plugin.Handler
	dockerEndpoint string
	dockerDomain   string
	dockerClient   *dockerapi.Client
	containerIPMap *containerIPMap
}

// NewDockerDiscovery constructs a new DockerDiscovery object
func NewDockerDiscovery(dockerEndpoint string, dockerDomain string) DockerDiscovery {
	return DockerDiscovery{
		dockerEndpoint: dockerEndpoint,
		dockerDomain:   dockerDomain,
		containerIPMap: &containerIPMap{
			make(map[string][]string),
			make(map[string]net.IP),
		},
	}
}

// ServeDNS implements plugin.Handler
func (dd DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r, Context: ctx}
	log.Println(dd.containerIPMap.byHostName)
	var answers []dns.RR
	switch state.QType() {
	case dns.TypeA:
		address, ok := dd.containerIPMap.byHostName[state.QName()]
		if ok {
			log.Printf("[docker] Found ip %v for host %s", address, state.QName())
			answers = a(state.Name(), []net.IP{address})
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
	m, _ = state.Scrub(m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements plugin.Handler
func (dd DockerDiscovery) Name() string {
	return "docker"
}

func (dd DockerDiscovery) getContainerAddress(container *dockerapi.Container) (net.IP, error) {
	log.Printf("[docker] Getting container address for %s\n", container.ID)
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
			log.Printf("Container %s is in another container's network namspace", container.ID)
			otherID := container.HostConfig.NetworkMode[len("container:")]
			var err error
			container, err = dd.dockerClient.InspectContainer(string(otherID))
			if err != nil {
				return nil, err
			}
			continue
		} else {
			network, ok := container.NetworkSettings.Networks[networkMode]
			if !ok {
				return nil, fmt.Errorf("Unable to find network settings for the network %s", networkMode)
			}
			return net.ParseIP(network.IPAddress), nil
		}
	}
}

func (dd DockerDiscovery) addContainer(containerID string) error {
	container, err := dd.dockerClient.InspectContainer(containerID)
	if err != nil {
		return err
	}
	containerAddress, err := dd.getContainerAddress(container)
	log.Printf("[docker] container %s has address %v", container.ID, containerAddress)
	if err != nil {
		return err
	}
	hostName := fmt.Sprintf("%s.%s", container.Config.Hostname, dd.dockerDomain)
	dd.containerIPMap.containerIDHostNameMap[containerID] = []string{hostName} // TODO: deal with multiple hostnames
	dd.containerIPMap.byHostName[hostName] = containerAddress
	return nil
}

func (dd DockerDiscovery) stopContainer(containerID string) error {
	hostnames, ok := dd.containerIPMap.containerIDHostNameMap[containerID]
	if !ok {
		log.Printf("[docker] No hostname associated with the container %s", containerID)
		return nil
	}
	for _, hostname := range hostnames {
		log.Printf("[docker] Deleting hostname entry %s", hostname)
		delete(dd.containerIPMap.byHostName, hostname)
	}
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

	for _, container := range containers {
		if err := dd.addContainer(container.ID); err != nil {
			log.Printf("[docker] Error adding A record for container %s: %s\n", container.ID, err)
		}
	}

	for msg := range events {
		go func(msg *dockerapi.APIEvents) {
			switch msg.Status {
			case "start":
				log.Println("[docker] New container spawned. Attempt to add A record for it")
				if err := dd.addContainer(msg.ID); err != nil {
					log.Printf("[docker] Error adding A record for container %s: %s", msg.ID, err)
				}
			case "die":
				log.Println("[docker] Container being stopped. Attempt to remove its A record from the DNS", msg.ID)
				if err := dd.stopContainer(msg.ID); err != nil {
					log.Printf("[docker] Error deleting A record for container: %s: %s", msg.ID, err)
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
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeA,
			Class: dns.ClassINET, Ttl: 3600}
		r.A = ip
		answers = append(answers, r)
	}
	return answers
}
