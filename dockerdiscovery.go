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
	byCName                map[string]string
	byIP                   map[string]string
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
		containerIPMap: &containerIPMap{},
	}
}

// ServeDNS implements plugin.Handler
func (dd DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r, Context: ctx}
	fmt.Println(state.Name())
	fmt.Println(state.QName())
	fmt.Println(state.QType())
	switch state.QType() {
	case dns.TypeA:
		address, ok := dd.containerIPMap.byHostName[state.QName()]
		if ok {
			m := new(dns.Msg)
			m.SetReply(r)
			m.Answer = []dns.RR{
				dns.A{A: address, Hdr: nil},
			}
		}
	}
	return dd.Next.ServeDNS(ctx, w, r)
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
				if err := dd.addContainer(msg.ID); err != nil {
					log.Printf("[docker] Error adding A record for container %s: %s", msg.ID, err)
				}
			case "die":
				log.Printf("[docker] Removing A record for container %s", msg.ID)
			}
		}(msg)
	}

	return errors.New("docker event loop closed")
}
