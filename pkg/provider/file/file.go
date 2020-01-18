package file

import (
	"context"

	"github.com/anabiozz/rproxy/pkg/config/dynamic"
	"github.com/anabiozz/rproxy/pkg/provider"
)

// Provider ..
type Provider struct {
	File string
}

// Provide ..
func (p Provider) Provide(ctx context.Context, cfg chan *dynamic.Configuration) (err error) {
	return nil
}

func init() {
	provider.Add("file", func() provider.Provider {
		return &Provider{}
	})
}
