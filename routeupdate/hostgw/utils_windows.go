package hostgw

import (
	"net"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
	winroute "github.com/rancher/win-route-netsh"
)

func (p *HostGw) getCurrentRouteEntries(iface net.Interface, host metadata.Host, subnet *net.IPNet) (map[string]*winroute.RouteRow, error) {
	routes, err := p.r.GetRoutes()
	if err != nil {
		return nil, err
	}
	routerIpAddress := host.Labels[routerIPLabel]
	routeEntries := make(map[string]*winroute.RouteRow)
	for _, route := range routes {
		// Skip routes on other interfaces
		if uint64(iface.Index) != route.InterfaceIndex {
			continue
		}
		//Skip ipv6 route
		if route.AddressFamily != "IPv4" {
			continue
		}

		if route.DestinationPrefix.Contains(net.ParseIP(routerIpAddress)) {
			continue
		}

		if route.NextHop.Equal(net.ParseIP("0.0.0.0")) {
			continue
		}

		gwIP := route.NextHop.String()
		routeEntries[gwIP] = route
	}

	p.logRouteEntries(routeEntries, "getCurrentRouteEntries")
	return routeEntries, nil
}

func (p *HostGw) getDesiredRouteEntries(iface net.Interface, selfHost metadata.Host, allHosts []metadata.Host) (map[string]*winroute.RouteRow, error) {
	routeEntries := make(map[string]*winroute.RouteRow)

	for _, h := range allHosts {
		// Link-local routes already in place
		if h.UUID == selfHost.UUID {
			continue
		}

		_, ipNet, err := net.ParseCIDR(h.Labels[perHostSubnetLabel])
		if err != nil {
			log.WithField("host", h.UUID).Warn(err)
			continue
		}

		privateNetworkIp, ok := h.Labels[routerIPLabel]
		if !ok {
			log.Warnf("host %s don't have lable $s, skip this host", h.UUID, routerIPLabel)
			continue
		}

		r := winroute.RouteRow{
			DestinationPrefix: ipNet,
			InterfaceIndex:    uint64(iface.Index),
			NextHop:           net.ParseIP(privateNetworkIp),
		}
		routeEntries[h.AgentIP] = &r
	}

	p.logRouteEntries(routeEntries, "getDesiredRouteEntries")
	return routeEntries, nil
}

func (p *HostGw) updateRoutes(oldEntries map[string]*winroute.RouteRow, newEntries map[string]*winroute.RouteRow) error {
	var e error

	for ip, oe := range oldEntries {
		ne, ok := newEntries[ip]

		if ok && oe.Equal(ne) {
			delete(newEntries, ip)
		} else {
			err := p.r.DeleteRouteByDest(oe.DestinationPrefix.String())
			if err != nil {
				log.Errorf("updateRoute: failed to DeleteRoute, %v", err)
				e = errors.Wrap(e, err.Error())
			}
		}
	}

	for _, ne := range newEntries {
		err := p.r.AddRoute(ne)
		if err != nil {
			log.Errorf("updateRoute: failed to AddRoute, %v", err)
			e = errors.Wrap(e, err.Error())
		}
	}

	return e
}

func (p *HostGw) logRouteEntries(entries map[string]*winroute.RouteRow, action string) {
	if log.GetLevel() == log.DebugLevel {
		for _, route := range entries {
			log.WithFields(log.Fields{
				"dest":    route.DestinationPrefix.String(),
				"gateway": route.NextHop,
			}).Debug(action)
		}
	}
}

func getInterfaceFromAddress(tar string) ([]net.Interface, error) {
	var rtn []net.Interface
	is, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, Interface := range is {
		addrs, err := Interface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			tmp := addr.String()
			if tar == strings.Split(tmp, "/")[0] {
				rtn = append(rtn, Interface)
			}
		}
	}
	return rtn, nil
}
