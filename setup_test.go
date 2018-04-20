package dockerdiscovery

import (
	"testing"

	"github.com/mholt/caddy"
	"github.com/stretchr/testify/assert"
)

type setupDockerDiscoveryTestCase struct {
	configBlock              string
	expectedDockerSocketPath string
	expectedDockerDomain     string
}

func TestSetupDockerDiscovery(t *testing.T) {
	testCases := []setupDockerDiscoveryTestCase{
		setupDockerDiscoveryTestCase{
			"docker",
			defaultDockerSocketPath,
			defaultDockerDomain,
		},
		setupDockerDiscoveryTestCase{
			"docker /var/run/docker.sock.backup",
			"/var/run/docker.sock.backup",
			defaultDockerDomain,
		},
		setupDockerDiscoveryTestCase{
			`docker {
	domain example.org.
}`,
			defaultDockerSocketPath,
			"example.org.",
		},
		setupDockerDiscoveryTestCase{
			`docker /home/user/docker.sock {
	domain home.example.org.
}`,
			"/home/user/docker.sock",
			"home.example.org.",
		},
	}

	for _, tc := range testCases {
		c := caddy.NewTestController("dns", tc.configBlock)
		dd, err := createPlugin(c)
		assert.Nil(t, err)
		assert.Equal(t, dd.dockerSocketPath, tc.expectedDockerSocketPath)
		assert.Equal(t, dd.dockerDomain, tc.expectedDockerDomain)
	}
}
