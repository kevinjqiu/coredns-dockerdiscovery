package dockerdiscovery

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"

	"github.com/mholt/caddy"
)

const defaultDockerSocketPath = "/var/run/docker.sock"
const defaultDockerDomain = "docker.local"

func init() {
	caddy.RegisterPlugin("docker", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func createPlugin(c *caddy.Controller) (DockerDiscovery, error) {
	dd := DockerDiscovery{
		dockerSocketPath: defaultDockerSocketPath,
		dockerDomain:     defaultDockerDomain,
	}

	for c.Next() {
		args := c.RemainingArgs()
		if len(args) == 1 {
			dd.dockerSocketPath = args[0]
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
	return dd, nil
}

func setup(c *caddy.Controller) error {
	dd, err := createPlugin(c)
	if err != nil {
		return err
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return dd
	})
	return nil
}
