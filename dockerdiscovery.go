package dockerdiscovery

import (
	"context"
	"errors"
	"log"

	"github.com/coredns/coredns/plugin"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/miekg/dns"
)

// DockerDiscovery is a plugin that conforms to the coredns plugin interface
type DockerDiscovery struct {
	Next           plugin.Handler
	dockerEndpoint string
	dockerDomain   string
	dockerClient   *dockerapi.Client
}

// ServeDNS implements plugin.Handler
func (dd DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return dd.Next.ServeDNS(ctx, w, r)
}

// Name implements plugin.Handler
func (dd DockerDiscovery) Name() string {
	return "docker"
}

func (dd DockerDiscovery) getContainerAddress(container *dockerapi.Container) (string, error) {
	return "", nil
}

func (dd DockerDiscovery) addContainer(containerID string) error {
	container, err := dd.dockerClient.InspectContainer(containerID)
	if err != nil {
		return err
	}
	containerAddress, err := dd.getContainerAddress(container)
	if err != nil {
		return err
	}
	return nil
}

func (dd DockerDiscovery) start() error {
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
			log.Printf("[docker] Error adding A record for container %s: %s", container.ID, err)
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
