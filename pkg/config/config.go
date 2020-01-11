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

// EntryPoint holds the entry point configuration.
type EntryPoint struct {
	Address string `toml:"address,omitempty"`
	// 	Transport        *EntryPointsTransport `toml:"transport,omitempty"`
	// 	ProxyProtocol    *ProxyProtocol        `toml:"proxyProtocol,omitempty" label:"allowEmpty"`
	// 	ForwardedHeaders *ForwardedHeaders     `toml:"forwardedHeaders,omitempty"`
}

// ProviderConfiguration ..
type ProviderConfiguration struct {
	Routers  map[string]*Router
	Services map[string]*Service
}

// Router ..
type Router struct {
	Service string
}

// Service ..
type Service struct {
	*LoadBalancer
	EntryPoints
}

// LoadBalancer ..
type LoadBalancer struct {
	Servers []Server
}

// Server ..
type Server struct {
	URL string
}
