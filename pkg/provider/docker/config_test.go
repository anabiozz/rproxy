package docker

import (
	"context"
	"testing"

	"github.com/anabiozz/rproxy/pkg/config/dynamic"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
)

func Int(v int) *int    { return &v }
func Bool(v bool) *bool { return &v }

func TestDefaultRule(t *testing.T) {
	testCases := []struct {
		desc       string
		containers []dockerData
		expected   *dynamic.Configuration
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
			expected: &dynamic.Configuration{
				Routers: map[string]*dynamic.Router{
					"Test": {
						Service: "Test",
					},
				},
				Services: map[string]*dynamic.Service{
					"Test": {
						LoadBalancer: &dynamic.LoadBalancer{
							Servers: []dynamic.Server{
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
