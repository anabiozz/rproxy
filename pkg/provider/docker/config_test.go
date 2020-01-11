package docker

import (
	"context"
	"testing"

	"github.com/anabiozz/rproxy/pkg/config"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func Int(v int) *int    { return &v }
func Bool(v bool) *bool { return &v }

func TestDefaultRule(t *testing.T) {
	testCases := []struct {
		desc       string
		containers []dockerData
		expected   *config.ProviderConfiguration
	}{
		{
			desc: "default container",
			containers: []dockerData{
				{
					ServiceName: "Test",
					Labels: map[string]string{
						"rpoxy.routers.container.host": "Test",
					},
					NetworkSettings: networkSettings{
						Ports: nat.PortMap{
							nat.Port("8080/tcp"): []nat.PortBinding{},
						},
						Networks: map[string]networkData{
							"bridge": {
								Name: "bridge",
								Addr: "127.0.0.1",
								Port: 8080,
							},
						},
						NetworkMode: "bridge",
					},
				},
			},
			expected: &config.ProviderConfiguration{
				Routers: map[string]*config.Router{
					"Test": {
						Service: "Test",
					},
				},
				Services: map[string]*config.Service{
					"Test": {
						EntryPoints: map[string]*config.EntryPoint{
							"Test": {
								Address: ":8080",
							},
						},
						LoadBalancer: &config.LoadBalancer{
							Servers: []config.Server{
								{
									URL: "http://127.0.0.1:8080",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			p := Provider{}

			configuration := p.buildConfiguration(context.Background(), test.containers)

			t.Log(configuration)

			assert.Equal(t, test.expected, configuration)
		})
	}
}
