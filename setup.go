package dockerdiscovery

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	dockerapi "github.com/fsouza/go-dockerclient"

	"github.com/mholt/caddy"
)

const defaultDockerEndpoint = "unix:///var/run/docker.sock"
const defaultDockerDomain = "docker.local."

func init() {
	caddy.RegisterPlugin("docker", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

// TODO(kevinjqiu): add docker endpoint verification
func createPlugin(c *caddy.Controller) (DockerDiscovery, error) {
	dd := NewDockerDiscovery(defaultDockerEndpoint, defaultDockerDomain)

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) == 1 {
			dd.dockerEndpoint = args[0]
		}

		if len(args) > 1 {
			return dd, c.ArgErr()
		}

		for c.NextBlock() {
			switch c.Val() {
			case "domain":
				if !c.NextArg() {
					return dd, c.ArgErr()
				}
				dd.dockerDomain = c.Val()
			default:
				return dd, c.Errf("unknown property: '%s'", c.Val())
			}
		}
	}
	dockerClient, err := dockerapi.NewClient(dd.dockerEndpoint)
	if err != nil {
		return dd, err
	}
	dd.dockerClient = dockerClient
	go dd.start()
	return dd, nil
}

func setup(c *caddy.Controller) error {
	dd, err := createPlugin(c)
	if err != nil {
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		dd.Next = next
		return dd
	})
	return nil
}
