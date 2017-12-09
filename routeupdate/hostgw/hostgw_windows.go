package hostgw

import (
	"fmt"
	"net"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
	winroute "github.com/rancher/win-route-netsh"
)

const (
	ProviderName = "hostgw"

	changeCheckInterval = 5
	perHostSubnetLabel  = "io.rancher.network.per_host_subnet.subnet"
	routerIPLabel       = "io.rancher.network.per_host_subnet.router_ip"
)

type HostGw struct {
	m metadata.Client
	r winroute.IRouter
}

func New(m metadata.Client) (*HostGw, error) {
	o := &HostGw{
		m: m,
		r: winroute.New(),
	}
	return o, nil
}

func (p *HostGw) Start() {
	go p.m.OnChange(changeCheckInterval, p.onChangeNoError)
}

// TODO p.r.Close() when a stop signal is received

func (p *HostGw) onChangeNoError(version string) {
	if err := p.Reload(); err != nil {
		log.Errorf("Failed to apply host route : %v", err)
	}
}

func (p *HostGw) Reload() error {
	log.Debug("HostGW: reload")
	if err := p.configure(); err != nil {
		return errors.Wrap(err, "Failed to reload hostgw routes")
	}
	return nil
}

func (p *HostGw) configure() error {
	log.Debug("HostGW: reload")

	selfHost, err := p.m.GetSelfHost()
	if err != nil {
		return errors.Wrap(err, "Failed to get self host from metadata")
	}
	allHosts, err := p.m.GetHosts()
	if err != nil {
		return errors.Wrap(err, "Failed to get all hosts from metadata")
	}
	_, ipNet, err := net.ParseCIDR(selfHost.Labels[perHostSubnetLabel])
	if err != nil {
		return errors.Wrapf(err, "Selfhost subnet configuration error")
	}

	routerIpAddress, ok := selfHost.Labels[routerIPLabel]
	if !ok {
		log.Warnf("this host %s don't have lable $s, skip this host", selfHost.UUID, routerIPLabel)
		return fmt.Errorf("this host %s don't have lable $s, skip this host", selfHost.UUID, routerIPLabel)
	}
	// interface indeces aren't static, do a lookup each pass
	Is, err := getInterfaceFromAddress(routerIpAddress)
	if err != nil {
		return errors.Wrap(err, "Failed to get Interface by agent ip")
	}
	//ToDo Locate the interface more precisely
	if len(Is) != 1 {
		return errors.New("")
	}

	currentRoutes, err := p.getCurrentRouteEntries(Is[0], selfHost, ipNet)
	if err != nil {
		return errors.Wrap(err, "Failed to getCurrentRouteEntries")
	}
	desiredRoutes, err := p.getDesiredRouteEntries(Is[0], selfHost, allHosts)
	if err != nil {
		return errors.Wrap(err, "Failed to getDesiredRouteEntries")
	}
	err = p.updateRoutes(currentRoutes, desiredRoutes)
	if err != nil {
		return errors.Wrap(err, "Failed to updateRoutes")
	}
	return err
}
