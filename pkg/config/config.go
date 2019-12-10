package config

// Configuration .
type Configuration struct {
	EntryPoints EntryPoints
	Providers   Providers
}

// EntryPoints holds the HTTP entry point list.
type EntryPoints map[string]*EntryPoint

// Providers ..
type Providers map[string]map[string]interface{}

// Providers contains providers configuration
// type Providers struct {
// 	Docker *docker.Provider `toml:"docker,omitempty" export:"true" label:"allowEmpty"`
// 	File   *file.Provider   `toml:"file,omitempty" export:"true" label:"allowEmpty"`
// }

// EntryPoint holds the entry point configuration.
type EntryPoint struct {
	Address string `toml:"address,omitempty"`
	// 	Transport        *EntryPointsTransport `toml:"transport,omitempty"`
	// 	ProxyProtocol    *ProxyProtocol        `toml:"proxyProtocol,omitempty" label:"allowEmpty"`
	// 	ForwardedHeaders *ForwardedHeaders     `toml:"forwardedHeaders,omitempty"`
}

// ProviderConfiguration is the root of the dynamic configuration
type ProviderConfiguration struct {
	Routers  map[string]Router  `toml:"routers,omitempty"`
	Services map[string]Service `toml:"services,omitempty"`
}

// Router holds the router configuration.
type Router struct {
	Service  string `toml:"service,omitempty"`
	Rule     string `toml:"rule,omitempty"`
	Priority int    `toml:"priority,omitempty,omitzero"`
}

// Service ..
type Service struct {
	Name string
	Path string
	Host string
}
