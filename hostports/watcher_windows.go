package hostports

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/orangedeng/go-winnat"

	"github.com/Sirupsen/logrus"
	natdrivers "github.com/orangedeng/go-winnat/drivers"
	"github.com/rancher/go-rancher-metadata/metadata"
)

const (
	reapplyEvery  = 5 * time.Minute
	networkName   = "transparent"
	routerIPLabel = "io.rancher.network.per_host_subnet.router_ip"
)

var (
	skipPrefixs = []string{"vEthernet", "isatap", "Loopback"}
)

type watcher struct {
	c                metadata.Client
	natdriver        winnat.NatDriver
	appliedPortRules map[string]natdrivers.PortMapping
	lastApplied      time.Time
}

func Watch(c metadata.Client) error {
	names, err := getNatInterfaceNames(c)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return errors.New("Get NAT interface error")
	}
	conf := map[string]interface{}{}
	conf[natdrivers.NatAdapterName] = strings.Join(names, ",")
	logrus.Debugf("driver config is %#v", conf)
	driver, err := winnat.NewNatDriver(natdrivers.NetshDriverName, conf)
	if err != nil {
		return err
	}
	w := &watcher{
		c:                c,
		natdriver:        driver,
		appliedPortRules: map[string]natdrivers.PortMapping{},
	}
	go c.OnChange(5, w.onChangeNoError)
	return nil
}

func (w *watcher) onChangeNoError(version string) {
	if err := w.onChange(version); err != nil {
		logrus.Errorf("Failed to apply ipset: %v", err)
	}
}

func (w *watcher) onChange(version string) error {
	logrus.Debug("hostports:  Creating rule set")
	newPortRules := map[string]natdrivers.PortMapping{}

	host, err := w.c.GetSelfHost()
	if err != nil {
		return err
	}

	networks, err := w.c.GetNetworks()
	if err != nil {
		return err
	}

	networkUUID, err := networkUUID(networks)
	if err != nil {
		return err
	}

	containers, err := w.c.GetContainers()
	if err != nil {
		return err
	}

	//Find containers assigned to this host and use per-host-subnet
	//And generate the port mapping rules
	for _, container := range containers {
		if container.HostUUID != host.UUID || container.NetworkUUID != networkUUID {
			continue
		}
		//if container is not running don't apply the rule
		if !(container.State == "running" || container.State == "starting" || container.State == "stopping") {
			continue
		}

		for _, port := range container.Ports {
			rule, ok := parsePortRule(host.AgentIP, container.PrimaryIp, port)
			if !ok {
				continue
			}

			newPortRules[container.ExternalId+"/"+port] = rule
		}
	}
	logrus.Debugf("hostports:  New generated rules: %v", newPortRules)
	if !compareRules(w.appliedPortRules, newPortRules) {
		logrus.Infof("hostports: : Applying new rules")
		return w.apply(newPortRules)
	} else if time.Now().Sub(w.lastApplied) > reapplyEvery {
		return w.apply(newPortRules)
	}
	return nil
}

func (w *watcher) apply(newRules map[string]natdrivers.PortMapping) error {
	defer func() {
		w.appliedPortRules = newRules
		w.lastApplied = time.Now()
	}()
	l, err := w.natdriver.ListPortMapping()
	if err != nil {
		return err
	}
	logrus.Infof("%#v", l)
	var rules []natdrivers.PortMapping
	for _, rule := range newRules {
		rules = append(rules, rule)
	}
	if err := w.natdriver.DeletePortMappings(l); err != nil {
		return errors.Wrap(err, "error when deleting current port mapping rules")
	}
	return w.natdriver.CreatePortMappings(rules)
}

func networkUUID(networks []metadata.Network) (string, error) {
	for _, network := range networks {
		if network.Name == networkName {
			return network.UUID, nil
		}
	}
	return "", errors.New("per-host-subnet not found")
}

func parsePortRule(externalIP, InternalIP string, portMapping string) (natdrivers.PortMapping, bool) {
	proto := "tcp"
	parts := strings.Split(portMapping, ":")
	if len(parts) != 3 {
		return natdrivers.PortMapping{}, false
	}

	sourceIP, _sourcePort, _targetPort := parts[0], parts[1], parts[2]

	parts = strings.Split(_targetPort, "/")
	if len(parts) == 2 {
		_targetPort = parts[0]
		proto = parts[1]
	}
	targetPort, _ := strconv.ParseUint(_targetPort, 10, 0)
	sourcePort, _ := strconv.ParseUint(_sourcePort, 10, 0)
	return natdrivers.PortMapping{
		Protocol:     proto,
		InternalIP:   net.ParseIP(InternalIP),
		InternalPort: uint32(targetPort),
		ExternalIP:   net.ParseIP(sourceIP),
		ExternalPort: uint32(sourcePort),
	}, true
}

func parseIPPortMap(input []*natdrivers.PortMapping) map[string]natdrivers.PortMapping {
	rtn := map[string]natdrivers.PortMapping{}
	for _, v := range input {
		rtn[v.ExternalIP.String()+":"+strconv.Itoa(int(v.ExternalPort))] = *v
	}
	return rtn
}

func compareRules(a, b map[string]natdrivers.PortMapping) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !(&v).Equal(&bv) {
			return false
		}
	}
	return true
}

func getNatInterfaceNames(c metadata.Client) ([]string, error) {
	selfHost, err := c.GetSelfHost()
	if err != nil {
		return nil, err
	}
	routerip, ok := selfHost.Labels[routerIPLabel]
	if !ok {
		return nil, fmt.Errorf("Missing rancher labels %s", routerIPLabel)
	}
	var rtn []string
	is, err := net.Interfaces()
	if err != nil {
		return []string{}, err
	}
outer:
	for _, i := range is {
		adds, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, add := range adds {
			if strings.Contains(add.String(), routerip) {
				continue outer
			}
		}
		for _, prefix := range skipPrefixs {
			if strings.Contains(i.Name, prefix) {
				continue outer
			}
		}
		rtn = append(rtn, i.Name)
	}
	return rtn, nil
}
