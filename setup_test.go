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

func TestConfigDockerDiscovery(t *testing.T) {
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
}

func TestSetupDockerDiscovery(t *testing.T) {
	networkName := "my_project_network_name"
	c := caddy.NewTestController("dns", fmt.Sprintf(`docker unix:///home/user/docker.sock {
	compose_domain compose.loc
	hostname_domain home.example.org
	domain docker.loc
	network_aliases %s
}`, networkName))
	dd, err := createPlugin(c)
	assert.Nil(t, err)

	var address = net.ParseIP("192.11.0.1")
	var containers = []*dockerapi.Container{
		genContainerDefn(address.String(), networkName, ""),
		genContainerDefn("", networkName, address.String()),
		genContainerDefn(address.String(), networkName, address.String()),
	}

	for i := range containers {
		container := containers[i]
		e := dd.updateContainerInfo(container)
		assert.Nil(t, e)

		_ = ipOk(t, dd, "myproject.loc.", address)
		ipNotOk(t, dd, "wrong.loc.")
		_ = ipOk(t, dd, "nginx.home.example.org.", address)
		ipNotOk(t, dd, "wrong.home.example.org.")
		_ = ipOk(t, dd, "label-host.loc.", address)
		_ = ipOk(t, dd, "cservice.cproject.compose.loc.", address)

		containerInfo := ipOk(t, dd, fmt.Sprintf("%s.docker.loc.", container.Name), address)
		assert.Equal(t, container.Name, containerInfo.container.Name)
	}
}

func TestMultipleNetworksDockerDiscovery(t *testing.T) {
	networkName := "my_project_network_name"
	address := net.ParseIP("192.11.0.1")
	expectedAddress := net.ParseIP("9.14.1.30")
	expectedNet := "inquisition"

	c := caddy.NewTestController("dns", fmt.Sprintf(`docker unix:///home/user/docker.sock {
	compose_domain compose.loc
	hostname_domain home.example.org
	domain docker.loc
	network_aliases %s
}`, networkName))
	dd, err := createPlugin(c)
	assert.Nil(t, err)

	// generate a configuration; tweak to add a second network
	container := genContainerDefn("", networkName, address.String())
	container.NetworkSettings.Networks[expectedNet] = dockerapi.ContainerNetwork{
		Aliases:   []string{"myproject.loc"},
		IPAddress: expectedAddress.String(),
	}

	err = dd.updateContainerInfo(container)
	assert.Nil(t, err)

	// without label, we expect the "NetworkMode" address to prevail
	_ = ipOk(t, dd, "label-host.loc.", address)

	// now, update for the label and try this again

	container.Config.Labels["coredns.dockerdiscovery.network"] = expectedNet
	err = dd.updateContainerInfo(container)
	assert.Nil(t, err)

	_ = ipOk(t, dd, "label-host.loc.", expectedAddress)

	return
}

// simple check
func ipOk(t *testing.T, dd *DockerDiscovery, domain string, address net.IP) *ContainerInfo {

	containerInfo, e := dd.containerInfoByDomain(domain)
	assert.Nil(t, e)
	assert.NotNil(t, containerInfo)

	// check as strings here, for us poor mortals
	assert.Equal(t, address.String(), containerInfo.address.String())

	return containerInfo
}

// simple check
func ipNotOk(t *testing.T, dd *DockerDiscovery, domain string) {

	containerInfo, e := dd.containerInfoByDomain(domain)
	assert.Nil(t, e)
	assert.Nil(t, containerInfo)

	return
}

// string, not net.IP, as 1) we're test, 2) the underling struct is a string,
// and 3) we may want something odd here
func genContainerDefn(nsAddress string, netMode string, netAddress string) *dockerapi.Container {
	container := &dockerapi.Container{
		ID:   "fa155d6fd141e29256c286070d2d44b3f45f1e46822578f1e7d66c1e7981e6c7",
		Name: "evil_ptolemy",
		Config: &dockerapi.Config{
			Hostname: "nginx",
			Labels: map[string]string{
				"coredns.dockerdiscovery.host": "label-host.loc",
				"com.docker.compose.project":   "cproject",
				"com.docker.compose.service":   "cservice",
			},
		},
		HostConfig: &dockerapi.HostConfig{
			NetworkMode: netMode,
		},
		NetworkSettings: &dockerapi.NetworkSettings{
			IPAddress: nsAddress,
			Networks: map[string]dockerapi.ContainerNetwork{
				netMode: dockerapi.ContainerNetwork{
					Aliases:   []string{"myproject.loc"},
					IPAddress: netAddress,
				},
			},
		},
	}

	return container
}
