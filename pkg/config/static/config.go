package static

import (
	"github.com/anabiozz/rproxy/pkg/provider/docker"
	"github.com/anabiozz/rproxy/pkg/provider/file"
)

// Configuration .
type Configuration struct {
	Providers   *Providers
	EntryPoints *EntryPoints
}

// EntryPoints holds the HTTP entry point list.
type EntryPoints map[string]*EntryPoint

// Providers ..
type Providers struct {
	Docker *docker.Provider
	File   *file.Provider
}

// EntryPoint holds the entry point configuration.
type EntryPoint struct {
	Address string `toml:"address,omitempty"`
}
