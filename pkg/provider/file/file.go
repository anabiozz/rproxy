package file

import (
	"context"

	"github.com/anabiozz/rproxy/pkg/config"
	"github.com/anabiozz/rproxy/pkg/provider"
)

// Provider ..
type Provider struct {
	File string
}

// Provide ..
func (p Provider) Provide(ctx context.Context, cfg chan *config.ProviderConfiguration) (err error) {
	return nil
}

func init() {
	provider.Add("file", func() provider.Provider {
		return &Provider{}
	})
}
