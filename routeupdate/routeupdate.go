package routeupdate

import (
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/rancher/per-host-subnet/routeupdate/hostgw"
)

type RouteUpdate interface {
	Start()
	Reload() error
}

func Run(provider string, m metadata.Client) (RouteUpdate, error) {
	switch provider {
	case hostgw.ProviderName:
		r, err := hostgw.New(m)
		if err != nil {
			return nil, err
		}
		r.Start()
		return r, nil
	default:
		return nil, errors.New("No provider specified")
	}
}
