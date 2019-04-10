package dockerdiscovery

import (
	dockerapi "github.com/fsouza/go-dockerclient"
	"fmt"
	"strings"
)

// resolvers implements ContainerDomainResolver

type SubDomainContainerNameResolver struct {
	domain string
}
func (resolver SubDomainContainerNameResolver) resolve(container *dockerapi.Container) ([]string, error) {
	var domains []string
	domains = append(domains, fmt.Sprintf("%s.%s", strings.TrimLeft(container.Name, "/"), resolver.domain))
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

	for label, value :=  range container.Config.Labels {
		if label == resolver.hostLabel {
			domains = append(domains, value)
			break;
		}
	}

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

