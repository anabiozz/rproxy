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
		desc        string
		containers  []dockerData
		defaultRule string
		expected    *config.ProviderConfiguration
	}{
		{
			desc: "default rule with no variable",
			containers: []dockerData{
				{
					ServiceName: "Test",
					Labels:      map[string]string{},
					NetworkSettings: networkSettings{
						Ports: nat.PortMap{
							nat.Port("80/tcp"): []nat.PortBinding{},
						},
						Networks: map[string]networkData{
							"bridge": {
								Name: "bridge",
								Addr: "127.0.0.1",
							},
						},
					},
				},
			},
			defaultRule: "Host(`foo.bar`)",
			expected: &config.ProviderConfiguration{
				Routers: map[string]config.Router{
					"Test": {
						Service: "Test",
						Rule:    "Host(`foo.bar`)",
					},
				},
				Services: map[string]config.Service{
					"Test": {
						// LoadBalancer: &config.ServersLoadBalancer{
						// 	Servers: []config.Server{
						// 		{
						// 			URL: "http://127.0.0.1:80",
						// 		},
						// 	},
						// 	PassHostHeader: Bool(true),
						// },
					},
				},
			},
		},
		{
			desc: "default rule with service name",
			containers: []dockerData{
				{
					ServiceName: "Test",
					Labels:      map[string]string{},
					NetworkSettings: networkSettings{
						Ports: nat.PortMap{
							nat.Port("80/tcp"): []nat.PortBinding{},
						},
						Networks: map[string]networkData{
							"bridge": {
								Name: "bridge",
								Addr: "127.0.0.1",
							},
						},
					},
				},
			},
			defaultRule: "Host(`{{ .Name }}.foo.bar`)",
			expected: &config.ProviderConfiguration{
				Routers: map[string]config.Router{
					"Test": {
						Service: "Test",
						Rule:    "Host(`Test.foo.bar`)",
					},
				},
				// Services: map[string]config.Service{
				// 	"Test": {
				// 		LoadBalancer: &config.ServersLoadBalancer{
				// 			Servers: []config.Server{
				// 				{
				// 					URL: "http://127.0.0.1:80",
				// 				},
				// 			},
				// 			PassHostHeader: Bool(true),
				// 		},
				// 	},
				// },
			},
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			p := Provider{
				ExposedByDefault: true,
				DefaultRule:      test.defaultRule,
			}

			configuration := p.buildConfiguration(context.Background(), test.containers)

			assert.Equal(t, test.expected, configuration)
		})
	}
}
