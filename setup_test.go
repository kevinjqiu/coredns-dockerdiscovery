package dockerdiscovery

import (
	"testing"

	"github.com/mholt/caddy"
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
	domain example.org.
}`,
			defaultDockerEndpoint,
			"example.org.",
		},
		setupDockerDiscoveryTestCase{
			`docker unix:///home/user/docker.sock {
	domain home.example.org.
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
		assert.Equal(t, dd.dockerDomain, tc.expectedDockerDomain)
	}
}
