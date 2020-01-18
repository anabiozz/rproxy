package provider

import (
	"context"

	"github.com/anabiozz/rproxy/pkg/config/dynamic"
)

// Provider ...
type Provider interface {
	Provide(ctx context.Context, providerConfiguration chan *dynamic.Configuration) error
}

// Creator ..
type Creator func() Provider

// Providers ..
var Providers = map[string]Creator{}

// Add ..
func Add(name string, creator Creator) {
	Providers[name] = creator
}
