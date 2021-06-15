package dockerdiscovery

import (
	"fmt"
	"net"
	"testing"

	"github.com/coredns/caddy"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

type setupDockerDiscoveryTestCase struct {
	configBlock            string
	expectedDockerEndpoint string
	expectedDockerDomain   string
}

func TestSetupDockerDiscovery(t *testing.T) {
	testCases := []setupDockerDiscoveryTestCase{
		setupDockerDiscoveryTestCase{
			"docker",
			defaultDockerEndpoint,
			defaultDockerDomain,
		},
		setupDockerDiscoveryTestCase{
			"docker unix:///var/run/docker.sock.backup",
			"unix:///var/run/docker.sock.backup",
			defaultDockerDomain,
		},
		setupDockerDiscoveryTestCase{
			`docker {
	hostname_domain example.org.
}`,
			defaultDockerEndpoint,
			"example.org.",
		},
		setupDockerDiscoveryTestCase{
			`docker unix:///home/user/docker.sock {
	hostname_domain home.example.org.
}`,
			"unix:///home/user/docker.sock",
			"home.example.org.",
		},
	}

	for _, tc := range testCases {
		c := caddy.NewTestController("dns", tc.configBlock)
		dd, err := createPlugin(c)
		assert.Nil(t, err)
		assert.Equal(t, dd.dockerEndpoint, tc.expectedDockerEndpoint)
	}

	c := caddy.NewTestController("dns", `docker unix:///home/user/docker.sock {
	hostname_domain home.example.org
	domain docker.loc
	network_aliases my_project_network_name
}`)
	dd, err := createPlugin(c)
	assert.Nil(t, err)

	networks := make(map[string]dockerapi.ContainerNetwork)
	var aliases = []string{"myproject.loc"}

	networks["my_project_network_name"] = dockerapi.ContainerNetwork{
		Aliases: aliases,
	}
	var address = net.ParseIP("192.11.0.1")
	var container = &dockerapi.Container{
		ID:   "fa155d6fd141e29256c286070d2d44b3f45f1e46822578f1e7d66c1e7981e6c7",
		Name: "evil_ptolemy",
		Config: &dockerapi.Config{
			Hostname: "nginx",
			Labels:   map[string]string{"coredns.dockerdiscovery.host": "label-host.loc"},
		},
		NetworkSettings: &dockerapi.NetworkSettings{
			Networks:  networks,
			IPAddress: address.String(),
		},
	}

	e := dd.updateContainerInfo(container)
	assert.Nil(t, e)

	containerInfo, e := dd.containerInfoByDomain("myproject.loc.")
	assert.Nil(t, e)
	assert.NotNil(t, containerInfo)
	assert.NotNil(t, containerInfo.address)

	assert.Equal(t, containerInfo.address, address)

	containerInfo, e = dd.containerInfoByDomain("wrong.loc.")
	assert.Nil(t, containerInfo)

	containerInfo, e = dd.containerInfoByDomain("nginx.home.example.org.")
	assert.NotNil(t, containerInfo)

	containerInfo, e = dd.containerInfoByDomain("wrong.home.example.org.")
	assert.Nil(t, containerInfo)

	containerInfo, e = dd.containerInfoByDomain("label-host.loc.")
	assert.NotNil(t, containerInfo)

	containerInfo, e = dd.containerInfoByDomain(fmt.Sprintf("%s.docker.loc.", container.Name))
	assert.NotNil(t, containerInfo)
	assert.Equal(t, container.Name, containerInfo.container.Name)
}
