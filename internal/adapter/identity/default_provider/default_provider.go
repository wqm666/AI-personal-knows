package default_provider

import (
	"context"

	"github.com/personal-know/internal/port"
)

type Provider struct {
	defaultOwner string
}

func New(defaultOwner string) *Provider {
	if defaultOwner == "" {
		defaultOwner = "default"
	}
	return &Provider{defaultOwner: defaultOwner}
}

func (p *Provider) Resolve(ctx context.Context) (*port.Identity, error) {
	return &port.Identity{
		OwnerID: p.defaultOwner,
		Name:    p.defaultOwner,
	}, nil
}
