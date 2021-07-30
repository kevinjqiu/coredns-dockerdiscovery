package dockerdiscovery

import (
	"fmt"
	dockerapi "github.com/fsouza/go-dockerclient"
	"log"
	"strings"
)

func normalizeContainerName(container *dockerapi.Container) string {
	return strings.TrimLeft(container.Name, "/")
}

// resolvers implements ContainerDomainResolver

type SubDomainContainerNameResolver struct {
	domain string
}

func (resolver SubDomainContainerNameResolver) resolve(container *dockerapi.Container) ([]string, error) {
	var domains []string
	domains = append(domains, fmt.Sprintf("%s.%s", normalizeContainerName(container), resolver.domain))
	return domains, nil
}

type SubDomainHostResolver struct {
	domain string
}

func (resolver SubDomainHostResolver) resolve(container *dockerapi.Container) ([]string, error) {
	var domains []string
	domains = append(domains, fmt.Sprintf("%s.%s", container.Config.Hostname, resolver.domain))
	return domains, nil
}

type LabelResolver struct {
	hostLabel string
}

func (resolver LabelResolver) resolve(container *dockerapi.Container) ([]string, error) {
	var domains []string

	for label, value := range container.Config.Labels {
		if label == resolver.hostLabel {
			domains = append(domains, value)
			break
		}
	}

	return domains, nil
}

// ComposeResolver sets names based on compose labels
type ComposeResolver struct {
	domain string
}

func (resolver ComposeResolver) resolve(container *dockerapi.Container) ([]string, error) {
	var domains []string

	project, pok := container.Config.Labels["com.docker.compose.project"]
	service, sok := container.Config.Labels["com.docker.compose.service"]
	if !pok || !sok {
		return domains, nil
	}

	domain := fmt.Sprintf("%s.%s.%s", service, project, resolver.domain)
	domains = append(domains, domain)

	log.Printf("[docker] Found compose domain for container %s: %s", container.ID[:12], domain)
	return domains, nil
}

type NetworkAliasesResolver struct {
	network string
}

func (resolver NetworkAliasesResolver) resolve(container *dockerapi.Container) ([]string, error) {
	var domains []string

	if resolver.network != "" {
		network, ok := container.NetworkSettings.Networks[resolver.network]
		if ok {
			domains = append(domains, network.Aliases...)
		}
	} else {
		for _, network := range container.NetworkSettings.Networks {
			domains = append(domains, network.Aliases...)
		}
	}

	return domains, nil
}
