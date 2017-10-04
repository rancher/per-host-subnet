package hostgw

import (
	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
)

const (
	ProviderName = "hostgw"

	changeCheckInterval = 5
	perHostSubnetLabel  = "io.rancher.network.per_host_subnet.subnet"
)

type HostGw struct {
	m metadata.Client
}

func New(m metadata.Client) (*HostGw, error) {
	o := &HostGw{
		m: m,
	}
	return o, nil
}

func (p *HostGw) Start() {
	go p.m.OnChange(changeCheckInterval, p.onChangeNoError)
}

func (p *HostGw) onChangeNoError(version string) {
	if err := p.Reload(); err != nil {
		logrus.Errorf("Failed to apply host route : %v", err)
	}
}

func (p *HostGw) Reload() error {
	logrus.Debug("HostGW: reload")
	if err := p.configure(); err != nil {
		return errors.Wrap(err, "Failed to reload hostgw routes")
	}
	return nil
}

func (p *HostGw) configure() error {
	logrus.Debug("HostGW: reload")

	selfHost, err := p.m.GetSelfHost()
	if err != nil {
		return errors.Wrap(err, "Failed to get self host from metadata")
	}
	allHosts, err := p.m.GetHosts()
	if err != nil {
		return errors.Wrap(err, "Failed to get all hosts from metadata")
	}

	currentRoutes, err := getCurrentRouteEntries(selfHost)
	if err != nil {
		return errors.Wrap(err, "Failed to getCurrentRouteEntries")
	}
	desiredRoutes, err := getDesiredRouteEntries(selfHost, allHosts)
	if err != nil {
		return errors.Wrap(err, "Failed to getDesiredRouteEntries")
	}
	err = updateRoutes(currentRoutes, desiredRoutes)
	if err != nil {
		return errors.Wrap(err, "Failed to updateRoutes")
	}
	return err
}
