package dynamic

// Configuration ..
type Configuration struct {
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
}

// LoadBalancer ..
type LoadBalancer struct {
	Servers []Server
}

// Server ..
type Server struct {
	URL string
}
