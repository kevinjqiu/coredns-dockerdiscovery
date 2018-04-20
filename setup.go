package dockerdiscovery

import (
	"fmt"

	"github.com/mholt/caddy"
)

const defaultDockerSocketPath = "/var/run/docker.sock"
const defaultDockerDomain = "docker.local"

func init() {
	caddy.RegisterPlugin("dockerdiscovery", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func createPlugin(c *caddy.Controller) (*DockerDiscovery, error) {
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
			return nil, c.ArgErr()
		}

		for c.NextBlock() {
			switch c.Val() {
			case "domain":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				dd.dockerDomain = c.Val()
			default:
				return nil, c.Errf("unknown property: '%s'", c.Val())
			}
		}
	}
	return &dd, nil
}

func setup(c *caddy.Controller) error {
	dd, err := createPlugin(c)
	if err != nil {
		return err
	}
	fmt.Println(dd)
	return nil
}
