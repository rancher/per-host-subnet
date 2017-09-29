package hostgw

import (
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher-metadata/metadata"
	"github.com/vishvananda/netlink"
)

const (
	agentIPLabel = "io.rancher.network.per_host_subnet.override_agent_ip"
)

func getHostSubnet(host metadata.Host) (*net.IPNet, error) {
	ipnet, err := netlink.ParseIPNet(host.Labels[perHostSubnetLabel])
	if err != nil {
		logrus.Errorf("Failed to parse host %s subnet: %s", host.Name, err)
	}
	return ipnet, err
}

func getCurrentRouteEntries(host metadata.Host) (map[string]*netlink.Route, error) {
	routeFilter := &netlink.Route{Src: net.ParseIP(getAgentIP(host))}
	existRoutes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_SRC)
	if err != nil {
		logrus.Errorf("Failed to getCurrentRouteEntries, RouteList: %v", err)
		return nil, err
	}

	routeEntries := make(map[string]*netlink.Route)
	for index, r := range existRoutes {
		if !r.Dst.Contains(net.ParseIP(getAgentIP(host))) {
			gwIP := r.Gw.String()
			routeEntries[gwIP] = &existRoutes[index]
		}
	}

	logrus.Debugf("getCurrentRouteEntries: routeEntries %v", routeEntries)
	return routeEntries, nil
}

func getDesiredRouteEntries(selfHost metadata.Host, allHosts []metadata.Host) (map[string]*netlink.Route, error) {
	routeEntries := make(map[string]*netlink.Route)

	for _, h := range allHosts {
		if h.UUID != selfHost.UUID {
			dst, err := getHostSubnet(h)
			if err != nil {
				return nil, err
			}
			r := &netlink.Route{
				Dst: dst,
				Src: net.ParseIP(getAgentIP(selfHost)),
				Gw:  net.ParseIP(getAgentIP(h)),
			}
			routeEntries[getAgentIP(h)] = r
		}
	}

	logrus.Debugf("getDesiredRouteEntries: routeEntries %v", routeEntries)
	return routeEntries, nil
}

func updateRoutes(oldEntries map[string]*netlink.Route, newEntries map[string]*netlink.Route) error {
	var e error

	for ip, oe := range oldEntries {
		_, ok := newEntries[ip]
		if ok {
			delete(newEntries, ip)
		} else {
			err := netlink.RouteDel(oe)
			if err != nil {
				logrus.Errorf("updateRoute: failed to RouteDel, %v", err)
				e = errors.Wrap(e, err.Error())
			}
		}
	}

	for _, ne := range newEntries {
		err := netlink.RouteAdd(ne)
		if err != nil {
			logrus.Errorf("updateRoute: failed to RouteAdd, %v", err)
			e = errors.Wrap(e, err.Error())
		}
	}

	return e
}

func getAgentIP(host metadata.Host) string {
	if v, ok := host.Labels[agentIPLabel]; ok {
		return v
	}
	return host.AgentIP
}
