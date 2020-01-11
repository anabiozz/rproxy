package docker

import (
	"context"
	"fmt"
	"strconv"
	"text/template"

	"github.com/anabiozz/rproxy/pkg/config"
	"github.com/anabiozz/rproxy/pkg/log"
	providerpkg "github.com/anabiozz/rproxy/pkg/provider"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/docker/docker/api/types"
	dockertypes "github.com/docker/docker/api/types"
	dockercontainertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// Provider ..
type Provider struct {
	Constraints    string
	Watch          bool
	Endpoint       string
	DefaultRule    string
	UseBindPortIP  bool
	SwarmMode      bool
	Network        string
	DetectionType  string `toml:"detectionType,omitempty"`
	defaultRuleTpl *template.Template
}

type dockerData struct {
	ID              string
	ServiceName     string
	Name            string
	Labels          map[string]string
	NetworkSettings networkSettings
	isRunning       bool
	Node            *dockertypes.ContainerNode // only for docker swarm
}

// NetworkSettings holds the networks data to the provider.
type networkSettings struct {
	NetworkMode dockercontainertypes.NetworkMode
	Ports       nat.PortMap
	Networks    map[string]networkData
}

// Network holds the network data to the provider.
type networkData struct {
	Name     string
	Addr     string
	Port     int
	Protocol string
	ID       string
}

func getNewDockerClient() (cli *client.Client, err error) {
	cli, err = client.NewEnvClient()
	if err != nil {
		return nil, fmt.Errorf("docker new client error: %v", err.Error())
	}
	return
}

// Provide ..
func (provider *Provider) Provide(
	providerCtx context.Context,
	providerConfigurationCh chan *config.ProviderConfiguration) (err error) {

	var providerConfiguration config.ProviderConfiguration
	providerConfiguration.Services = make(map[string]*config.Service)
	var _dockerData dockerData
	var _dockerDataArray []dockerData
	var _networkData networkData
	var _networkSettings networkSettings

	ctxLog := log.NewContext(providerCtx, log.Str(log.ProviderName, "docker"))
	logger := log.WithContext(ctxLog)

	ctx, cancel := context.WithCancel(ctxLog)
	defer cancel()

	cli, err := getNewDockerClient()
	if err != nil {
		return err
	}

	serverVersion, err := cli.ServerVersion(ctx)
	if err != nil {
		logger.Errorf("Failed to retrieve information of the docker: %s", err)
		return err
	}
	logger.Printf("Provider connection established with docker %s (API %s)\n", serverVersion.Version, serverVersion.APIVersion)

	// netName := rproxy.GetEnv("RPROXY_DOCKER_NET", "")
	// if netName == "" {
	// 	return errors.New("env RPROXY_DOCKER_NET should be not empty")
	// }

	containersFilters := filters.NewArgs()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: containersFilters})
	if err != nil {
		return err
	}

	for _, container := range containers {

		if container.Labels["rpoxy.routers.container.host"] == "" {
			continue
		}

		_dockerData.ID = container.ID
		_dockerData.Labels = container.Labels
		_dockerData.Name = container.Labels["com.docker.compose.service"]
		_dockerData.ServiceName = container.Labels["rpoxy.routers.container.host"]
		if container.State == "Running" {
			_dockerData.isRunning = true
		}

		if container.Ports != nil && len(container.Ports) > 0 {
			_networkData.Port = int(container.Ports[0].PublicPort)
			_networkData.Protocol = container.Ports[0].Type
			_networkData.Addr = container.Ports[0].IP
		}

		_networkSettings.Networks = make(map[string]networkData)

		for networkMode, networkSetting := range container.NetworkSettings.Networks {
			_networkData.ID = networkSetting.NetworkID
			_networkData.Name = networkMode
			_networkSettings.NetworkMode = dockercontainertypes.NetworkMode(networkMode)
			_networkSettings.Networks[networkMode] = _networkData
		}

		_dockerData.NetworkSettings = _networkSettings

		_dockerDataArray = append(_dockerDataArray, _dockerData)
	}

	providerConfigurationCh <- provider.buildConfiguration(ctx, _dockerDataArray)

	return nil
}

func (provider Provider) buildConfiguration(
	ctx context.Context,
	dockerDataArray []dockerData) (providerConfiguration *config.ProviderConfiguration) {

	// cxtLog := log.NewContext(ctx, log.Str(log.ProviderName, "docker.buildConfiguration"))
	// logger := log.WithContext(cxtLog)

	providerConfiguration = &config.ProviderConfiguration{}

	providerConfiguration.Services = make(map[string]*config.Service)
	providerConfiguration.Routers = make(map[string]*config.Router)

	for _, _dockerData := range dockerDataArray {

		_server := config.Server{}
		_service := &config.Service{}
		_router := &config.Router{}
		_service.LoadBalancer = &config.LoadBalancer{}

		_addr := _dockerData.NetworkSettings.Networks[string(_dockerData.NetworkSettings.NetworkMode)].Addr
		_port := _dockerData.NetworkSettings.Networks[string(_dockerData.NetworkSettings.NetworkMode)].Port

		_server.URL = "http://" + _addr + ":" + strconv.Itoa(_port)

		_router.Service = _dockerData.ServiceName

		if service, ok := providerConfiguration.Services[_dockerData.ServiceName]; ok {
			service.LoadBalancer.Servers = append(service.LoadBalancer.Servers, _server)
			_service = service
		} else {
			_service.LoadBalancer.Servers = append(_service.LoadBalancer.Servers, _server)
		}

		if _, ok := providerConfiguration.Routers[_dockerData.ServiceName]; ok {
			continue
		}

		providerConfiguration.Services[_dockerData.ServiceName] = _service
		providerConfiguration.Routers[_dockerData.ServiceName] = _router
	}

	return providerConfiguration
}

func init() {
	providerpkg.Add("docker", func() providerpkg.Provider {
		return &Provider{}
	})
}
