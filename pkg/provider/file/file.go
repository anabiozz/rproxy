package file

import (
	"context"

	"github.com/anabiozz/rproxy/pkg/config/dynamic"
	"github.com/anabiozz/rproxy/pkg/log"
	"github.com/anabiozz/rproxy/pkg/provider"
)

// Provider ..
type Provider struct {
	File      string
	Endpoints []*Endpoint
}

// Endpoint ..
type Endpoint struct {
	Name       string `toml:"name"`
	Localaddr  string `toml:"localaddr"`
	Remoteaddr string `toml:"remoteaddr"`
}

// Provide ..
func (p *Provider) Provide(providerCtx context.Context, cfg chan *dynamic.Configuration) (err error) {

	ctxLog := log.NewContext(providerCtx, log.Str(log.ProviderName, "file"))
	logger := log.WithContext(ctxLog)

	for _, endpoint := range p.Endpoints {
		logger.Info(endpoint)
	}

	return nil
}

func init() {
	provider.Add("file", func() provider.Provider {
		return &Provider{}
	})
}
