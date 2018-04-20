package dockerdiscovery

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/miekg/dns"
)

// DockerDiscovery is a plugin that conforms to the coredns plugin interface
type DockerDiscovery struct {
	Next           plugin.Handler
	dockerEndpoint string
	dockerDomain   string
}

// ServeDNS implements plugin.Handler
func (dd DockerDiscovery) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return 0, nil
}

// Name implements plugin.Handler
func (dd DockerDiscovery) Name() string {
	return "docker"
}
